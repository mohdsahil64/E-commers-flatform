// Package cart manages a customer's cart and wishlist. Prices shown are
// snapshots; the authoritative price is always recomputed at checkout by the
// orders module.
package cart

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/carlitos/backend/internal/httpx"
)

// Repository provides cart/wishlist persistence.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository builds a cart Repository.
func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// Item is a cart line with resolved product/listing display data.
type Item struct {
	ID              string  `json:"id"`
	SellerListingID string  `json:"seller_listing_id"`
	ProductID       string  `json:"product_id"`
	ProductTitle    string  `json:"product_title"`
	Slug            string  `json:"slug"`
	SKU             string  `json:"sku"`
	Image           string  `json:"image"`
	SellerName      string  `json:"seller_name"`
	Quantity        int     `json:"quantity"`
	UnitPrice       float64 `json:"unit_price"`
	Available       int     `json:"available_quantity"`
	LineTotal       float64 `json:"line_total"`
	// SellerCODEnabled mirrors the seller's COD setting so the storefront can
	// disable Cash on Delivery at checkout for items from a seller who turned
	// it off.
	SellerCODEnabled bool `json:"seller_cod_enabled"`
}

// customerCartID resolves (and lazily creates) the cart id for a customer user.
func (r *Repository) customerCartID(ctx context.Context, userID string) (string, error) {
	var cartID string
	err := r.pool.QueryRow(ctx, `
		SELECT c.id FROM carts c
		JOIN customers cu ON cu.id = c.customer_id
		WHERE cu.user_id = $1`, userID).Scan(&cartID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", httpx.NotFound("cart_not_found", "Cart not found")
	}
	return cartID, err
}

// CustomerID resolves the customer id for a user.
func (r *Repository) CustomerID(ctx context.Context, userID string) (string, error) {
	var id string
	err := r.pool.QueryRow(ctx, `SELECT id FROM customers WHERE user_id = $1`, userID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", httpx.NotFound("customer_not_found", "Customer not found")
	}
	return id, err
}

// List returns the resolved items in a customer's cart.
func (r *Repository) List(ctx context.Context, userID string) ([]Item, error) {
	cartID, err := r.customerCartID(ctx, userID)
	if err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `
		SELECT ci.id, ci.seller_listing_id, p.id, p.title, p.slug, sl.sku,
		       coalesce((SELECT file_key FROM product_images WHERE product_id = p.id ORDER BY is_primary DESC, sort_order LIMIT 1),''),
		       s.store_name, ci.quantity, sl.selling_price,
		       coalesce(inv.available_quantity,0), coalesce(s.cod_enabled, TRUE)
		FROM cart_items ci
		JOIN seller_listings sl ON sl.id = ci.seller_listing_id
		JOIN products p ON p.id = sl.product_id
		JOIN sellers s ON s.id = sl.seller_id
		LEFT JOIN inventory inv ON inv.seller_listing_id = sl.id
		WHERE ci.cart_id = $1
		ORDER BY ci.added_at DESC`, cartID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Item
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ID, &it.SellerListingID, &it.ProductID, &it.ProductTitle,
			&it.Slug, &it.SKU, &it.Image, &it.SellerName, &it.Quantity, &it.UnitPrice,
			&it.Available, &it.SellerCODEnabled); err != nil {
			return nil, err
		}
		it.LineTotal = it.UnitPrice * float64(it.Quantity)
		out = append(out, it)
	}
	return out, rows.Err()
}

// AddItem adds (or increments) a listing in the customer's cart.
func (r *Repository) AddItem(ctx context.Context, userID, listingID string, qty int) error {
	if qty <= 0 {
		qty = 1
	}
	cartID, err := r.customerCartID(ctx, userID)
	if err != nil {
		return err
	}
	// Snapshot the current price.
	var price float64
	if err := r.pool.QueryRow(ctx,
		`SELECT selling_price FROM seller_listings WHERE id = $1 AND status = 'active'`,
		listingID).Scan(&price); errors.Is(err, pgx.ErrNoRows) {
		return httpx.NotFound("listing_not_found", "Product listing not available")
	} else if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO cart_items (cart_id, seller_listing_id, quantity, unit_price_snapshot)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (cart_id, seller_listing_id)
		DO UPDATE SET quantity = cart_items.quantity + EXCLUDED.quantity,
		              unit_price_snapshot = EXCLUDED.unit_price_snapshot`,
		cartID, listingID, qty, price)
	return err
}

// UpdateQuantity sets the absolute quantity for a cart item (removes if <= 0).
func (r *Repository) UpdateQuantity(ctx context.Context, userID, listingID string, qty int) error {
	cartID, err := r.customerCartID(ctx, userID)
	if err != nil {
		return err
	}
	if qty <= 0 {
		_, err = r.pool.Exec(ctx,
			`DELETE FROM cart_items WHERE cart_id = $1 AND seller_listing_id = $2`, cartID, listingID)
		return err
	}
	_, err = r.pool.Exec(ctx,
		`UPDATE cart_items SET quantity = $3 WHERE cart_id = $1 AND seller_listing_id = $2`,
		cartID, listingID, qty)
	return err
}

// RemoveItem removes a listing from the cart.
func (r *Repository) RemoveItem(ctx context.Context, userID, listingID string) error {
	cartID, err := r.customerCartID(ctx, userID)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx,
		`DELETE FROM cart_items WHERE cart_id = $1 AND seller_listing_id = $2`, cartID, listingID)
	return err
}

// Clear empties a customer's cart (used post-checkout, in a tx).
func (r *Repository) ClearTx(ctx context.Context, tx pgx.Tx, cartID string) error {
	_, err := tx.Exec(ctx, `DELETE FROM cart_items WHERE cart_id = $1`, cartID)
	return err
}

// ClearSelectedTx removes only the given listings from the cart (used after a
// partial checkout where the customer selected a subset of items). If listingIDs
// is empty it clears the whole cart, matching ClearTx's whole-cart semantics so
// callers can pass the exact set that was ordered.
func (r *Repository) ClearSelectedTx(ctx context.Context, tx pgx.Tx, cartID string, listingIDs []string) error {
	if len(listingIDs) == 0 {
		return r.ClearTx(ctx, tx, cartID)
	}
	_, err := tx.Exec(ctx,
		`DELETE FROM cart_items WHERE cart_id = $1 AND seller_listing_id = ANY($2::text[])`,
		cartID, listingIDs)
	return err
}

// CartIDForCustomer returns the cart id for a customer id.
func (r *Repository) CartIDForCustomer(ctx context.Context, customerID string) (string, error) {
	var id string
	err := r.pool.QueryRow(ctx, `SELECT id FROM carts WHERE customer_id = $1`, customerID).Scan(&id)
	return id, err
}
