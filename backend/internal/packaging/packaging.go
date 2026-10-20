// Package packaging implements CARLITOS packaging management: admin controls
// central packaging inventory and issues stock to sellers; sellers request and
// consume packaging. Every movement is written to an immutable ledger.
package packaging

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/carlitos/backend/internal/httpx"
)

// Item is a packaging SKU with central stock.
type Item struct {
	ID                string `json:"id"`
	SKU               string `json:"sku"`
	Name              string `json:"name"`
	Type              string `json:"type"`
	Unit              string `json:"unit"`
	AvailableQuantity int    `json:"available_quantity"`
	IssuedQuantity    int    `json:"issued_quantity"`
	IsActive          bool   `json:"is_active"`
}

// Request is a seller packaging request.
type Request struct {
	ID                string    `json:"id"`
	RequestNumber     string    `json:"request_number"`
	SellerID          string    `json:"seller_id"`
	PackagingItemID   string    `json:"packaging_item_id"`
	ItemName          string    `json:"item_name"`
	RequestedQuantity int       `json:"requested_quantity"`
	ApprovedQuantity  *int      `json:"approved_quantity"`
	Status            string    `json:"status"`
	CreatedAt         time.Time `json:"created_at"`
}

// Balance is a seller's remaining packaging for an item.
type Balance struct {
	PackagingItemID string `json:"packaging_item_id"`
	ItemName        string `json:"item_name"`
	Type            string `json:"type"`
	IssuedQuantity  int    `json:"issued_quantity"`
	UsedQuantity    int    `json:"used_quantity"`
	Remaining       int    `json:"remaining"`
}

// Repository provides packaging persistence.
type Repository struct{ pool *pgxpool.Pool }

// NewRepository builds a packaging Repository.
func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// Service holds packaging business logic.
type Service struct{ repo *Repository }

// NewService builds a packaging Service.
func NewService(repo *Repository) *Service { return &Service{repo: repo} }

// ---- admin: items & inventory ----

// ListItems returns packaging items with central stock.
func (s *Service) ListItems(ctx context.Context) ([]Item, error) {
	rows, err := s.repo.pool.Query(ctx, `
		SELECT pi.id, pi.sku, pi.name, pi.type, pi.unit,
		       coalesce(inv.available_quantity,0), coalesce(inv.issued_quantity,0), pi.is_active
		FROM packaging_items pi
		LEFT JOIN packaging_inventory inv ON inv.packaging_item_id = pi.id
		ORDER BY pi.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Item
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ID, &it.SKU, &it.Name, &it.Type, &it.Unit,
			&it.AvailableQuantity, &it.IssuedQuantity, &it.IsActive); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// CreateItem creates a packaging SKU and its inventory row.
func (s *Service) CreateItem(ctx context.Context, sku, name, typ, unit string) (string, error) {
	if sku == "" || name == "" || typ == "" {
		return "", httpx.BadRequest("validation_failed", "sku, name and type are required")
	}
	var id string
	err := pgx.BeginFunc(ctx, s.repo.pool, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `
			INSERT INTO packaging_items (sku, name, type, unit)
			VALUES ($1,$2,$3,COALESCE(NULLIF($4,''),'piece')) RETURNING id`,
			sku, name, typ, unit).Scan(&id); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO packaging_inventory (packaging_item_id) VALUES ($1)`, id)
		return err
	})
	return id, err
}

// AddStock increases central available stock and records a ledger entry.
func (s *Service) AddStock(ctx context.Context, itemID string, qty int, adminID string) error {
	if qty <= 0 {
		return httpx.BadRequest("validation_failed", "Quantity must be positive")
	}
	return pgx.BeginFunc(ctx, s.repo.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			`UPDATE packaging_inventory SET available_quantity = available_quantity + $2 WHERE packaging_item_id = $1`,
			itemID, qty); err != nil {
			return err
		}
		return ledger(ctx, tx, itemID, nil, "stock_in", qty, adminID, "manual", nil)
	})
}

// ---- seller: requests ----

// CreateRequest lets a seller request packaging.
func (s *Service) CreateRequest(ctx context.Context, sellerID, itemID string, qty int) (string, error) {
	if qty <= 0 {
		return "", httpx.BadRequest("validation_failed", "Quantity must be positive")
	}
	var id string
	err := s.repo.pool.QueryRow(ctx, `
		INSERT INTO packaging_requests (request_number, seller_id, packaging_item_id, requested_quantity)
		VALUES ($1,$2,$3,$4) RETURNING id`,
		reqNumber(), sellerID, itemID, qty).Scan(&id)
	return id, err
}

// ListRequests returns requests, optionally filtered by seller.
func (s *Service) ListRequests(ctx context.Context, sellerID, status string) ([]Request, error) {
	args := []any{}
	conds := ""
	i := 0
	add := func(cond string, v any) {
		i++
		args = append(args, v)
		if conds == "" {
			conds = "WHERE " + fmt.Sprintf(cond, i)
		} else {
			conds += " AND " + fmt.Sprintf(cond, i)
		}
	}
	if sellerID != "" {
		add("pr.seller_id = $%d", sellerID)
	}
	if status != "" {
		add("pr.status = $%d", status)
	}
	rows, err := s.repo.pool.Query(ctx, `
		SELECT pr.id, pr.request_number, pr.seller_id, pr.packaging_item_id, pi.name,
		       pr.requested_quantity, pr.approved_quantity, pr.status, pr.created_at
		FROM packaging_requests pr
		JOIN packaging_items pi ON pi.id = pr.packaging_item_id `+conds+`
		ORDER BY pr.created_at DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Request
	for rows.Next() {
		var r Request
		if err := rows.Scan(&r.ID, &r.RequestNumber, &r.SellerID, &r.PackagingItemID,
			&r.ItemName, &r.RequestedQuantity, &r.ApprovedQuantity, &r.Status, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ApproveRequest approves+fulfils a request: deducts central stock, issues to
// the seller, updates the seller balance, and records ledger entries — all atomic.
func (s *Service) ApproveRequest(ctx context.Context, requestID string, approvedQty int, adminID string) error {
	return pgx.BeginFunc(ctx, s.repo.pool, func(tx pgx.Tx) error {
		var sellerID, itemID string
		var reqQty int
		var status string
		err := tx.QueryRow(ctx,
			`SELECT seller_id, packaging_item_id, requested_quantity, status
			 FROM packaging_requests WHERE id=$1 FOR UPDATE`, requestID).
			Scan(&sellerID, &itemID, &reqQty, &status)
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.NotFound("request_not_found", "Packaging request not found")
		}
		if err != nil {
			return err
		}
		if status != "pending" {
			return httpx.Conflict("already_processed", "Request already processed")
		}
		if approvedQty <= 0 {
			approvedQty = reqQty
		}

		// Deduct from central stock (conditional to avoid negative).
		ct, err := tx.Exec(ctx, `
			UPDATE packaging_inventory
			SET available_quantity = available_quantity - $2,
			    issued_quantity = issued_quantity + $2
			WHERE packaging_item_id = $1 AND available_quantity >= $2`, itemID, approvedQty)
		if err != nil {
			return err
		}
		if ct.RowsAffected() == 0 {
			return httpx.Conflict("insufficient_packaging", "Not enough central packaging stock")
		}

		if _, err := tx.Exec(ctx, `
			UPDATE packaging_requests SET status='fulfilled', approved_quantity=$2,
			       reviewed_by=$3, reviewed_at=now() WHERE id=$1`,
			requestID, approvedQty, adminID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO packaging_issues (request_id, seller_id, packaging_item_id, issued_quantity, issued_by)
			VALUES ($1,$2,$3,$4,$5)`, requestID, sellerID, itemID, approvedQty, adminID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO packaging_seller_balances (seller_id, packaging_item_id, issued_quantity)
			VALUES ($1,$2,$3)
			ON CONFLICT (seller_id, packaging_item_id)
			DO UPDATE SET issued_quantity = packaging_seller_balances.issued_quantity + EXCLUDED.issued_quantity`,
			sellerID, itemID, approvedQty); err != nil {
			return err
		}
		return ledger(ctx, tx, itemID, &sellerID, "issue", approvedQty, adminID, "request", &requestID)
	})
}

// RejectRequest rejects a pending request.
func (s *Service) RejectRequest(ctx context.Context, requestID, adminID string) error {
	ct, err := s.repo.pool.Exec(ctx,
		`UPDATE packaging_requests SET status='rejected', reviewed_by=$2, reviewed_at=now()
		 WHERE id=$1 AND status='pending'`, requestID, adminID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return httpx.NotFound("request_not_found", "Pending request not found")
	}
	return nil
}

// RecordUsage lets a seller record packaging consumption.
func (s *Service) RecordUsage(ctx context.Context, sellerID, itemID string, qty int) error {
	if qty <= 0 {
		return httpx.BadRequest("validation_failed", "Quantity must be positive")
	}
	return pgx.BeginFunc(ctx, s.repo.pool, func(tx pgx.Tx) error {
		ct, err := tx.Exec(ctx, `
			UPDATE packaging_seller_balances
			SET used_quantity = used_quantity + $3
			WHERE seller_id = $1 AND packaging_item_id = $2
			  AND (issued_quantity - used_quantity) >= $3`, sellerID, itemID, qty)
		if err != nil {
			return err
		}
		if ct.RowsAffected() == 0 {
			return httpx.Conflict("insufficient_balance", "Not enough packaging remaining")
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO packaging_usage (seller_id, packaging_item_id, used_quantity, reference_type)
			VALUES ($1,$2,$3,'manual')`, sellerID, itemID, qty); err != nil {
			return err
		}
		return ledger(ctx, tx, itemID, &sellerID, "use", -qty, "", "manual", nil)
	})
}

// SellerBalances returns a seller's remaining packaging per item.
func (s *Service) SellerBalances(ctx context.Context, sellerID string) ([]Balance, error) {
	rows, err := s.repo.pool.Query(ctx, `
		SELECT b.packaging_item_id, pi.name, pi.type, b.issued_quantity, b.used_quantity,
		       (b.issued_quantity - b.used_quantity) AS remaining
		FROM packaging_seller_balances b
		JOIN packaging_items pi ON pi.id = b.packaging_item_id
		WHERE b.seller_id = $1 ORDER BY pi.name`, sellerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Balance
	for rows.Next() {
		var b Balance
		if err := rows.Scan(&b.PackagingItemID, &b.ItemName, &b.Type, &b.IssuedQuantity,
			&b.UsedQuantity, &b.Remaining); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// ledger appends an immutable packaging ledger entry.
func ledger(ctx context.Context, tx pgx.Tx, itemID string, sellerID *string, entryType string, delta int, createdBy, refType string, refID *string) error {
	var creator *string
	if createdBy != "" {
		creator = &createdBy
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO packaging_ledger (packaging_item_id, seller_id, entry_type, quantity_delta, reference_type, reference_id, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		itemID, sellerID, entryType, delta, refType, refID, creator)
	return err
}

func reqNumber() string {
	return fmt.Sprintf("PKG-%d", time.Now().UnixNano()%1_000_000_000)
}
