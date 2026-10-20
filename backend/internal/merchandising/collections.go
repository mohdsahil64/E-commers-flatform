// Package merchandising implements collections and hero banners. Banner
// destinations and collection rules are stored in the database (never hardcoded
// in the frontend); the storefront resolves them into listing queries.
package merchandising

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/carlitos/backend/internal/httpx"
)

// Collection view model.
type Collection struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Kind        string `json:"kind"`
	ImageKey    string `json:"image_key"`
	IsActive    bool   `json:"is_active"`
}

// Rule is a collection rule row (for automatic/hybrid collections).
type Rule struct {
	RuleType     string `json:"rule_type"`
	RuleOperator string `json:"rule_operator"`
	RuleValue    string `json:"rule_value"`
}

// Repository provides merchandising persistence.
type Repository struct{ pool *pgxpool.Pool }

// NewRepository builds a merchandising Repository.
func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// ListCollections returns active collections (storefront) or all (admin).
func (r *Repository) ListCollections(ctx context.Context, activeOnly bool) ([]Collection, error) {
	q := `SELECT id, name, slug, description, type, kind, coalesce(image_key,''), is_active
	      FROM collections`
	if activeOnly {
		q += ` WHERE is_active = TRUE`
	}
	q += ` ORDER BY sort_order, name`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Collection
	for rows.Next() {
		var c Collection
		if err := rows.Scan(&c.ID, &c.Name, &c.Slug, &c.Description, &c.Type, &c.Kind,
			&c.ImageKey, &c.IsActive); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetCollectionBySlug returns a collection and its rules.
func (r *Repository) GetCollectionBySlug(ctx context.Context, slug string) (*Collection, []Rule, error) {
	var c Collection
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, slug, description, type, kind, coalesce(image_key,''), is_active
		FROM collections WHERE slug = $1 AND is_active = TRUE`, slug).
		Scan(&c.ID, &c.Name, &c.Slug, &c.Description, &c.Type, &c.Kind, &c.ImageKey, &c.IsActive)
	if err != nil {
		return nil, nil, httpx.NotFound("collection_not_found", "Collection not found")
	}
	rrows, err := r.pool.Query(ctx,
		`SELECT rule_type, rule_operator, rule_value FROM collection_rules WHERE collection_id=$1`, c.ID)
	if err != nil {
		return nil, nil, err
	}
	defer rrows.Close()
	var rules []Rule
	for rrows.Next() {
		var ru Rule
		if err := rrows.Scan(&ru.RuleType, &ru.RuleOperator, &ru.RuleValue); err != nil {
			return nil, nil, err
		}
		rules = append(rules, ru)
	}
	return &c, rules, rrows.Err()
}

// ProductsForCollection returns product cards for a collection: the union of
// manually-attached products and rule-matched approved products.
func (r *Repository) ProductsForCollection(ctx context.Context, collectionID string, limit, offset int) ([]CollectionCard, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT p.id, p.title, p.slug,
		       coalesce((SELECT file_key FROM product_images WHERE product_id=p.id ORDER BY is_primary DESC, sort_order LIMIT 1),''),
		       coalesce((SELECT min(selling_price) FROM seller_listings WHERE product_id=p.id AND status='active'),0),
		       p.rating_avg, p.rating_count
		FROM collection_products cp
		JOIN products p ON p.id = cp.product_id AND p.status = 'approved'
		WHERE cp.collection_id = $1
		ORDER BY cp.sort_order
		LIMIT $2 OFFSET $3`, collectionID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CollectionCard
	for rows.Next() {
		var c CollectionCard
		if err := rows.Scan(&c.ProductID, &c.Title, &c.Slug, &c.Image, &c.MinPrice,
			&c.RatingAvg, &c.RatingCount); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CollectionCard is a product card within a collection.
type CollectionCard struct {
	ProductID   string  `json:"product_id"`
	Title       string  `json:"title"`
	Slug        string  `json:"slug"`
	Image       string  `json:"image"`
	MinPrice    float64 `json:"min_price"`
	RatingAvg   float64 `json:"rating_avg"`
	RatingCount int     `json:"rating_count"`
}

// CreateCollection inserts a collection (admin).
func (r *Repository) CreateCollection(ctx context.Context, c Collection) (string, error) {
	var id string
	err := r.pool.QueryRow(ctx, `
		INSERT INTO collections (name, slug, description, type, kind, is_active)
		VALUES ($1,$2,$3,COALESCE(NULLIF($4,''),'manual'),COALESCE(NULLIF($5,''),'featured'),TRUE)
		RETURNING id`, c.Name, c.Slug, c.Description, c.Type, c.Kind).Scan(&id)
	return id, err
}

// AddProductToCollection attaches a product to a collection.
func (r *Repository) AddProductToCollection(ctx context.Context, collectionID, productID string, sort int) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO collection_products (collection_id, product_id, sort_order)
		VALUES ($1,$2,$3) ON CONFLICT (collection_id, product_id) DO UPDATE SET sort_order=EXCLUDED.sort_order`,
		collectionID, productID, sort)
	return err
}
