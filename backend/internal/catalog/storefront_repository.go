package catalog

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// StorefrontRepository serves the public catalogue: search, filter, sort. It
// only ever returns approved products with active seller listings, and always
// preserves seller/category/gender/price/rating/ranking on each result.
type StorefrontRepository struct {
	pool *pgxpool.Pool
}

// NewStorefrontRepository builds a StorefrontRepository.
func NewStorefrontRepository(pool *pgxpool.Pool) *StorefrontRepository {
	return &StorefrontRepository{pool: pool}
}

// ListingFilter captures the storefront query parameters.
type ListingFilter struct {
	Search       string
	GenderCode   string
	CategorySlug string
	BrandSlug    string
	SellerSlug   string
	CollectionSlug string
	MinPrice     *float64
	MaxPrice     *float64
	InStock      bool
	Season       string // '' | summer | winter (all == no filter)
	Sort         string // relevance | price_asc | price_desc | newest | rating | popularity
	Limit        int
	Offset       int
}

// ListingCard is a lightweight product card for grids/listings. It carries the
// cheapest active seller listing so price/seller are always present.
type ListingCard struct {
	ProductID    string  `json:"product_id"`
	Title        string  `json:"title"`
	Slug         string  `json:"slug"`
	BrandName    string  `json:"brand_name"`
	PrimaryImage string  `json:"primary_image"`
	MinPrice     float64 `json:"min_price"`
	MRP          float64 `json:"mrp"`
	RatingAvg    float64 `json:"rating_avg"`
	RatingCount  int     `json:"rating_count"`
	SellerID     string  `json:"seller_id"`
	SellerName   string  `json:"seller_name"`
	SellerSlug   string  `json:"seller_slug"`
	RankScore    float64 `json:"rank_score"`
	AvailableQty int     `json:"available_quantity"`
	InStock      bool    `json:"in_stock"`
}

// Search returns product cards matching the filter, plus the total count.
//
// Only approved products with at least one active seller listing are returned.
// The cheapest active listing per product determines the displayed price and
// seller. Ranking is read from the precomputed product_ranking_snapshots so the
// storefront never runs the heavy scoring calculation per request.
func (r *StorefrontRepository) Search(ctx context.Context, f ListingFilter) ([]ListingCard, int64, error) {
	// Build dynamic WHERE clauses with positional args.
	conds := []string{"p.status = 'approved'", "sl.status = 'active'"}
	args := []any{}
	add := func(clause string, val any) {
		args = append(args, val)
		conds = append(conds, fmt.Sprintf(clause, len(args)))
	}

	if s := strings.TrimSpace(f.Search); s != "" {
		// Combine full-text and trigram similarity for fuzzy matching.
		args = append(args, s)
		idx := len(args)
		conds = append(conds, fmt.Sprintf(
			"(p.search_tsv @@ plainto_tsquery('simple', $%d) OR p.title %% $%d)", idx, idx))
	}
	if f.GenderCode != "" {
		add("p.gender_id = (SELECT id FROM genders WHERE code = $%d)", f.GenderCode)
	}
	if f.CategorySlug != "" {
		args = append(args, f.CategorySlug)
		idx := len(args)
		// Match either the top category or the subcategory by slug.
		conds = append(conds, fmt.Sprintf(
			"(p.category_id IN (SELECT id FROM categories WHERE slug = $%d) OR p.subcategory_id IN (SELECT id FROM categories WHERE slug = $%d))", idx, idx))
	}
	if f.BrandSlug != "" {
		add("p.brand_id = (SELECT id FROM brands WHERE slug = $%d)", f.BrandSlug)
	}
	if f.SellerSlug != "" {
		add("s.store_slug = $%d", f.SellerSlug)
	}
	if f.MinPrice != nil {
		add("sl.selling_price >= $%d", *f.MinPrice)
	}
	if f.MaxPrice != nil {
		add("sl.selling_price <= $%d", *f.MaxPrice)
	}
	if f.InStock {
		conds = append(conds, "COALESCE(inv.available_quantity,0) > 0")
	}
	if f.Season != "" && f.Season != "all" {
		// Match the requested season OR season-agnostic ('all') products.
		add("(p.season = $%d OR p.season = 'all')", f.Season)
	}

	where := strings.Join(conds, " AND ")

	orderBy := "rank_score DESC NULLS LAST, p.created_at DESC"
	switch f.Sort {
	case "price_asc":
		orderBy = "min_price ASC"
	case "price_desc":
		orderBy = "min_price DESC"
	case "newest":
		orderBy = "p.created_at DESC"
	case "rating":
		orderBy = "p.rating_avg DESC, p.rating_count DESC"
	case "popularity":
		orderBy = "rank_score DESC NULLS LAST"
	}

	limit := f.Limit
	if limit <= 0 || limit > 100 {
		limit = 24
	}

	// Base query uses a lateral to pick the cheapest active listing per product.
	base := `
	FROM products p
	JOIN LATERAL (
		SELECT sl.* FROM seller_listings sl
		WHERE sl.product_id = p.id AND sl.status = 'active'
		ORDER BY sl.selling_price ASC LIMIT 1
	) sl ON TRUE
	JOIN sellers s ON s.id = sl.seller_id AND s.status = 'approved'
	LEFT JOIN inventory inv ON inv.seller_listing_id = sl.id
	LEFT JOIN product_ranking_snapshots rk ON rk.product_id = p.id
	WHERE ` + where

	// Count.
	var total int64
	countQ := "SELECT count(*) " + base
	if err := r.pool.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Page.
	args = append(args, limit, f.Offset)
	selectQ := `
	SELECT p.id, p.title, p.slug, coalesce((SELECT name FROM brands WHERE id = p.brand_id),''),
	       coalesce((SELECT file_key FROM product_images WHERE product_id = p.id ORDER BY is_primary DESC, sort_order LIMIT 1),''),
	       sl.selling_price AS min_price, sl.mrp,
	       p.rating_avg, p.rating_count,
	       s.id, s.store_name, s.store_slug,
	       coalesce(rk.score,0) AS rank_score,
	       COALESCE(inv.available_quantity,0) AS available_qty ` + base +
		fmt.Sprintf(" ORDER BY %s LIMIT $%d OFFSET $%d", orderBy, len(args)-1, len(args))

	rows, err := r.pool.Query(ctx, selectQ, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []ListingCard
	for rows.Next() {
		var c ListingCard
		if err := rows.Scan(&c.ProductID, &c.Title, &c.Slug, &c.BrandName, &c.PrimaryImage,
			&c.MinPrice, &c.MRP, &c.RatingAvg, &c.RatingCount,
			&c.SellerID, &c.SellerName, &c.SellerSlug, &c.RankScore, &c.AvailableQty); err != nil {
			return nil, 0, err
		}
		c.InStock = c.AvailableQty > 0
		out = append(out, c)
	}
	return out, total, rows.Err()
}
