// Package inventory manages stock with safe, atomic reservation to prevent
// overselling. All mutations go through conditional UPDATEs and row locking so
// concurrent checkouts cannot reserve the same unit twice. The backend is the
// sole authority on stock — frontend values are never trusted.
package inventory

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrInsufficientStock is returned when a reservation cannot be satisfied.
var ErrInsufficientStock = errors.New("insufficient stock")

// Levels is the current stock snapshot for a seller listing.
type Levels struct {
	SellerListingID   string `json:"seller_listing_id"`
	AvailableQuantity int    `json:"available_quantity"`
	ReservedQuantity  int    `json:"reserved_quantity"`
	DamagedQuantity   int    `json:"damaged_quantity"`
	ReturnedQuantity  int    `json:"returned_quantity"`
	LowStockThreshold int    `json:"low_stock_threshold"`
}

// Repository provides inventory persistence.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository builds an inventory Repository.
func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// Pool exposes the pool for callers that coordinate their own transactions
// (e.g. order creation reserves several listings within one tx).
func (r *Repository) Pool() *pgxpool.Pool { return r.pool }

// EnsureRow creates the inventory row for a listing if missing.
func (r *Repository) EnsureRow(ctx context.Context, listingID string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO inventory (seller_listing_id) VALUES ($1)
		 ON CONFLICT (seller_listing_id) DO NOTHING`, listingID)
	return err
}

// Get returns current stock levels for a listing.
func (r *Repository) Get(ctx context.Context, listingID string) (*Levels, error) {
	var l Levels
	err := r.pool.QueryRow(ctx, `
		SELECT seller_listing_id, available_quantity, reserved_quantity,
		       damaged_quantity, returned_quantity, low_stock_threshold
		FROM inventory WHERE seller_listing_id = $1`, listingID).
		Scan(&l.SellerListingID, &l.AvailableQuantity, &l.ReservedQuantity,
			&l.DamagedQuantity, &l.ReturnedQuantity, &l.LowStockThreshold)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, pgx.ErrNoRows
	}
	return &l, err
}

// ReserveTx atomically moves `qty` units from available -> reserved for a
// listing, within the provided transaction. It uses a conditional UPDATE so a
// concurrent transaction cannot double-reserve. Returns ErrInsufficientStock
// if not enough available stock exists.
//
// The row is first locked with SELECT ... FOR UPDATE to serialize concurrent
// reservations against the same listing.
func (r *Repository) ReserveTx(ctx context.Context, tx pgx.Tx, listingID string, qty int) error {
	if qty <= 0 {
		return errors.New("quantity must be positive")
	}
	// Lock the row to serialize concurrent reservations.
	var available int
	err := tx.QueryRow(ctx,
		`SELECT available_quantity FROM inventory WHERE seller_listing_id = $1 FOR UPDATE`,
		listingID).Scan(&available)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInsufficientStock
	}
	if err != nil {
		return err
	}
	if available < qty {
		return ErrInsufficientStock
	}
	ct, err := tx.Exec(ctx, `
		UPDATE inventory
		SET available_quantity = available_quantity - $2,
		    reserved_quantity  = reserved_quantity + $2
		WHERE seller_listing_id = $1 AND available_quantity >= $2`, listingID, qty)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrInsufficientStock
	}
	return r.logTx(ctx, tx, listingID, "reserve", -qty, available-qty, "order", nil)
}

// ReleaseTx moves reserved stock back to available (e.g. cancellation before
// shipment).
func (r *Repository) ReleaseTx(ctx context.Context, tx pgx.Tx, listingID string, qty int) error {
	if qty <= 0 {
		return nil
	}
	var newAvail int
	err := tx.QueryRow(ctx, `
		UPDATE inventory
		SET reserved_quantity  = GREATEST(reserved_quantity - $2, 0),
		    available_quantity = available_quantity + $2
		WHERE seller_listing_id = $1
		RETURNING available_quantity`, listingID, qty).Scan(&newAvail)
	if err != nil {
		return err
	}
	return r.logTx(ctx, tx, listingID, "release", qty, newAvail, "order", nil)
}

// CommitSaleTx consumes reserved stock permanently (on shipment/delivery). It
// reduces reserved_quantity without returning to available.
func (r *Repository) CommitSaleTx(ctx context.Context, tx pgx.Tx, listingID string, qty int) error {
	if qty <= 0 {
		return nil
	}
	var avail int
	err := tx.QueryRow(ctx, `
		UPDATE inventory
		SET reserved_quantity = GREATEST(reserved_quantity - $2, 0)
		WHERE seller_listing_id = $1
		RETURNING available_quantity`, listingID, qty).Scan(&avail)
	if err != nil {
		return err
	}
	return r.logTx(ctx, tx, listingID, "sale", -qty, avail, "order", nil)
}

// Adjust sets an absolute available quantity (admin/seller manual adjustment)
// and records the delta in the ledger.
func (r *Repository) Adjust(ctx context.Context, listingID string, newAvailable int, note string, actorID *string) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		var current int
		err := tx.QueryRow(ctx,
			`SELECT available_quantity FROM inventory WHERE seller_listing_id = $1 FOR UPDATE`,
			listingID).Scan(&current)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`UPDATE inventory SET available_quantity = $2 WHERE seller_listing_id = $1`,
			listingID, newAvailable); err != nil {
			return err
		}
		return r.logTx(ctx, tx, listingID, "adjustment", newAvailable-current, newAvailable, "manual", actorID)
	})
}

// Restock increases available quantity by qty.
func (r *Repository) Restock(ctx context.Context, listingID string, qty int, note string, actorID *string) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		var avail int
		err := tx.QueryRow(ctx, `
			UPDATE inventory SET available_quantity = available_quantity + $2
			WHERE seller_listing_id = $1 RETURNING available_quantity`,
			listingID, qty).Scan(&avail)
		if err != nil {
			return err
		}
		return r.logTx(ctx, tx, listingID, "restock", qty, avail, "manual", actorID)
	})
}

// History returns the recent ledger entries for a listing.
func (r *Repository) History(ctx context.Context, listingID string, limit int) ([]Transaction, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, `
		SELECT change_type, quantity_delta, balance_after, reference_type,
		       coalesce(note,''), created_at
		FROM inventory_transactions
		WHERE seller_listing_id = $1
		ORDER BY created_at DESC
		LIMIT $2`, listingID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Transaction
	for rows.Next() {
		var t Transaction
		if err := rows.Scan(&t.ChangeType, &t.QuantityDelta, &t.BalanceAfter,
			&t.ReferenceType, &t.Note, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// logTx appends an immutable inventory ledger entry.
func (r *Repository) logTx(ctx context.Context, tx pgx.Tx, listingID, changeType string, delta, balanceAfter int, refType string, actorID *string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO inventory_transactions
			(seller_listing_id, change_type, quantity_delta, balance_after, reference_type, created_by)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		listingID, changeType, delta, balanceAfter, refType, actorID)
	return err
}
