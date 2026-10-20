package cart

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/carlitos/backend/internal/httpx"
)

// WishlistItem is a resolved wishlist entry.
type WishlistItem struct {
	ProductID    string  `json:"product_id"`
	Title        string  `json:"title"`
	Slug         string  `json:"slug"`
	Image        string  `json:"image"`
	MinPrice     float64 `json:"min_price"`
	RatingAvg    float64 `json:"rating_avg"`
}

func (r *Repository) wishlistID(ctx context.Context, userID string) (string, error) {
	var id string
	err := r.pool.QueryRow(ctx, `
		SELECT w.id FROM wishlists w
		JOIN customers cu ON cu.id = w.customer_id
		WHERE cu.user_id = $1`, userID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", httpx.NotFound("wishlist_not_found", "Wishlist not found")
	}
	return id, err
}

// ListWishlist returns the customer's wishlist items.
func (r *Repository) ListWishlist(ctx context.Context, userID string) ([]WishlistItem, error) {
	wid, err := r.wishlistID(ctx, userID)
	if err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `
		SELECT p.id, p.title, p.slug,
		       coalesce((SELECT file_key FROM product_images WHERE product_id = p.id ORDER BY is_primary DESC, sort_order LIMIT 1),''),
		       coalesce((SELECT min(selling_price) FROM seller_listings WHERE product_id = p.id AND status='active'),0),
		       p.rating_avg
		FROM wishlist_items wi
		JOIN products p ON p.id = wi.product_id
		WHERE wi.wishlist_id = $1
		ORDER BY wi.added_at DESC`, wid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WishlistItem
	for rows.Next() {
		var it WishlistItem
		if err := rows.Scan(&it.ProductID, &it.Title, &it.Slug, &it.Image, &it.MinPrice, &it.RatingAvg); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// AddWishlist adds a product to the wishlist.
func (r *Repository) AddWishlist(ctx context.Context, userID, productID string) error {
	wid, err := r.wishlistID(ctx, userID)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO wishlist_items (wishlist_id, product_id) VALUES ($1,$2)
		ON CONFLICT (wishlist_id, product_id) DO NOTHING`, wid, productID)
	return err
}

// RemoveWishlist removes a product from the wishlist.
func (r *Repository) RemoveWishlist(ctx context.Context, userID, productID string) error {
	wid, err := r.wishlistID(ctx, userID)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx,
		`DELETE FROM wishlist_items WHERE wishlist_id = $1 AND product_id = $2`, wid, productID)
	return err
}

var _ = context.Background
