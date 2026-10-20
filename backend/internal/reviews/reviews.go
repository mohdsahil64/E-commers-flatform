// Package reviews implements product reviews & ratings. A customer may review a
// product only once per eligible (delivered) order item; ratings roll up onto
// the product and seller. Admins can moderate.
package reviews

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/carlitos/backend/internal/httpx"
)

// Review is a product review view model.
type Review struct {
	ID                 string   `json:"id"`
	ProductID          string   `json:"product_id"`
	CustomerName       string   `json:"customer_name"`
	Rating             int      `json:"rating"`
	Title              string   `json:"title"`
	Body               string   `json:"body"`
	IsVerifiedPurchase bool     `json:"is_verified_purchase"`
	Status             string   `json:"status"`
	CreatedAt          string   `json:"created_at"`
	Photos             []string `json:"photos"`
}

// maxReviewPhotos caps how many photos may be attached to a single review.
const maxReviewPhotos = 5

// Repository provides review persistence.
type Repository struct{ pool *pgxpool.Pool }

// NewRepository builds a reviews Repository.
func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// Service wraps review business rules.
type Service struct{ repo *Repository }

// NewService builds a reviews Service.
func NewService(repo *Repository) *Service { return &Service{repo: repo} }

// ListForProduct returns published reviews for a product, each with its photos.
func (s *Service) ListForProduct(ctx context.Context, productID string, limit, offset int) ([]Review, error) {
	rows, err := s.repo.pool.Query(ctx, `
		SELECT r.id, r.product_id, coalesce(cu.display_name,'Customer'), r.rating,
		       r.title, r.body, r.is_verified_purchase, r.status, r.created_at::text
		FROM reviews r
		JOIN customers cu ON cu.id = r.customer_id
		WHERE r.product_id = $1 AND r.status = 'published'
		ORDER BY r.created_at DESC LIMIT $2 OFFSET $3`, productID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Review
	index := map[string]int{}
	for rows.Next() {
		var r Review
		if err := rows.Scan(&r.ID, &r.ProductID, &r.CustomerName, &r.Rating, &r.Title,
			&r.Body, &r.IsVerifiedPurchase, &r.Status, &r.CreatedAt); err != nil {
			return nil, err
		}
		index[r.ID] = len(out)
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := s.attachPhotos(ctx, out, index); err != nil {
		return nil, err
	}
	return out, nil
}

// attachPhotos loads review_images for a page of reviews in one batched query
// (avoids N+1) and assigns the file keys back to each review in sort order.
func (s *Service) attachPhotos(ctx context.Context, list []Review, index map[string]int) error {
	if len(list) == 0 {
		return nil
	}
	ids := make([]string, len(list))
	for i := range list {
		ids[i] = list[i].ID
	}
	rows, err := s.repo.pool.Query(ctx, `
		SELECT review_id, file_key FROM review_images
		WHERE review_id = ANY($1) ORDER BY review_id, sort_order`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var reviewID, fileKey string
		if err := rows.Scan(&reviewID, &fileKey); err != nil {
			return err
		}
		if i, ok := index[reviewID]; ok {
			list[i].Photos = append(list[i].Photos, fileKey)
		}
	}
	return rows.Err()
}

// CreateInput is the review submission payload.
type CreateInput struct {
	UserID      string
	OrderItemID string
	Rating      int
	Title       string
	Body        string
	PhotoKeys   []string
}

// Create records a review for an eligible, delivered order item and updates the
// product/seller rating rollups. Duplicate reviews are blocked by the unique
// constraint on order_item_id.
func (s *Service) Create(ctx context.Context, in CreateInput) (string, error) {
	if in.Rating < 1 || in.Rating > 5 {
		return "", httpx.BadRequest("validation_failed", "Rating must be between 1 and 5")
	}

	var reviewID string
	err := pgx.BeginFunc(ctx, s.repo.pool, func(tx pgx.Tx) error {
		// Verify the order item belongs to this customer and is delivered.
		var productID, sellerID, customerID string
		var delivered bool
		err := tx.QueryRow(ctx, `
			SELECT sl.product_id, oi.seller_id, cu.id,
			       (oi.item_status = 'delivered')
			FROM order_items oi
			JOIN orders o ON o.id = oi.order_id
			JOIN customers cu ON cu.id = o.customer_id
			JOIN seller_listings sl ON sl.id = oi.seller_listing_id
			WHERE oi.id = $1 AND cu.user_id = $2`, in.OrderItemID, in.UserID).
			Scan(&productID, &sellerID, &customerID, &delivered)
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.Forbidden("not_eligible", "You can only review items you purchased")
		}
		if err != nil {
			return err
		}
		if !delivered {
			return httpx.BadRequest("not_delivered", "You can review a product after it is delivered")
		}

		err = tx.QueryRow(ctx, `
			INSERT INTO reviews (product_id, customer_id, seller_id, order_item_id, rating,
				title, body, is_verified_purchase, status)
			VALUES ($1,$2,$3,$4,$5,$6,$7,TRUE,'published')
			RETURNING id`,
			productID, customerID, sellerID, in.OrderItemID, in.Rating, in.Title, in.Body).
			Scan(&reviewID)
		if err != nil {
			// Unique violation => already reviewed.
			return httpx.Conflict("already_reviewed", "You have already reviewed this item")
		}

		// Attach up to maxReviewPhotos photos (storage keys from the uploader).
		order := 0
		for _, key := range in.PhotoKeys {
			if key == "" {
				continue
			}
			if order >= maxReviewPhotos {
				break
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO review_images (review_id, file_key, sort_order)
				VALUES ($1,$2,$3)`, reviewID, key, order); err != nil {
				return err
			}
			order++
		}

		// Recompute product rating rollup.
		if _, err := tx.Exec(ctx, `
			UPDATE products SET
				rating_avg = COALESCE((SELECT round(avg(rating)::numeric,2) FROM reviews WHERE product_id=$1 AND status='published'),0),
				rating_count = (SELECT count(*) FROM reviews WHERE product_id=$1 AND status='published')
			WHERE id=$1`, productID); err != nil {
			return err
		}
		// Recompute seller rating rollup.
		_, err = tx.Exec(ctx, `
			UPDATE sellers SET
				rating_avg = COALESCE((SELECT round(avg(rating)::numeric,2) FROM reviews WHERE seller_id=$1 AND status='published'),0),
				rating_count = (SELECT count(*) FROM reviews WHERE seller_id=$1 AND status='published')
			WHERE id=$1`, sellerID)
		return err
	})
	return reviewID, err
}

// Moderate sets a review's status (admin).
func (s *Service) Moderate(ctx context.Context, reviewID, status, adminID string) error {
	if status != "published" && status != "rejected" && status != "hidden" {
		return httpx.BadRequest("validation_failed", "Invalid moderation status")
	}
	ct, err := s.repo.pool.Exec(ctx,
		`UPDATE reviews SET status=$2, moderated_by=$3, moderated_at=now() WHERE id=$1`,
		reviewID, status, adminID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return httpx.NotFound("review_not_found", "Review not found")
	}
	return nil
}

// ListForModeration returns reviews for admin moderation.
func (s *Service) ListForModeration(ctx context.Context, status string, limit, offset int) ([]Review, error) {
	if status == "" {
		status = "published"
	}
	rows, err := s.repo.pool.Query(ctx, `
		SELECT r.id, r.product_id, coalesce(cu.display_name,'Customer'), r.rating,
		       r.title, r.body, r.is_verified_purchase, r.status, r.created_at::text
		FROM reviews r JOIN customers cu ON cu.id = r.customer_id
		WHERE r.status = $1 ORDER BY r.created_at DESC LIMIT $2 OFFSET $3`,
		status, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Review
	for rows.Next() {
		var r Review
		if err := rows.Scan(&r.ID, &r.ProductID, &r.CustomerName, &r.Rating, &r.Title,
			&r.Body, &r.IsVerifiedPurchase, &r.Status, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
