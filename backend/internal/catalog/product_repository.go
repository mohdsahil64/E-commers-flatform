package catalog

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound indicates a missing catalog record.
var ErrNotFound = errors.New("not found")

// ProductRepository provides product / variant / listing persistence.
type ProductRepository struct {
	pool *pgxpool.Pool
}

// NewProductRepository builds a ProductRepository.
func NewProductRepository(pool *pgxpool.Pool) *ProductRepository {
	return &ProductRepository{pool: pool}
}

// Pool exposes the pool for multi-step transactional creation.
func (r *ProductRepository) Pool() *pgxpool.Pool { return r.pool }

// CreateProductInput captures the fields to create a product + its variants.
type CreateProductInput struct {
	SellerID      string
	Title         string
	Slug          string
	Description   string
	BrandID       *string
	GenderID      *string
	CategoryID    *string
	SubcategoryID *string
	HSNCode       string
	TaxPercent    float64
	Season        string
	Attributes    map[string]any
}

// CreateProduct inserts a product row in 'draft' status and returns its id.
func (r *ProductRepository) CreateProduct(ctx context.Context, tx pgx.Tx, in CreateProductInput) (string, error) {
	season := in.Season
	if season == "" {
		season = "all"
	}
	var id string
	q := `INSERT INTO products
	        (created_by_seller, title, slug, description, brand_id, gender_id,
	         category_id, subcategory_id, hsn_code, tax_percent, season, attributes, status)
	      VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,COALESCE($12,'{}'::jsonb),'draft')
	      RETURNING id`
	run := func(row pgx.Row) error { return row.Scan(&id) }
	if tx != nil {
		err := run(tx.QueryRow(ctx, q, in.SellerID, in.Title, in.Slug, in.Description,
			in.BrandID, in.GenderID, in.CategoryID, in.SubcategoryID, in.HSNCode, in.TaxPercent, season, in.Attributes))
		return id, err
	}
	err := run(r.pool.QueryRow(ctx, q, in.SellerID, in.Title, in.Slug, in.Description,
		in.BrandID, in.GenderID, in.CategoryID, in.SubcategoryID, in.HSNCode, in.TaxPercent, season, in.Attributes))
	return id, err
}

// UpdateProductInput holds the editable fields of a seller's product.
type UpdateProductInput struct {
	Title       string
	Description string
	BrandID     *string
	GenderID    *string
	CategoryID  *string
	Season      string
}

// UpdateSellerProduct updates a product's editable fields, scoped to the owning
// seller. Editing is only allowed while the product is not live-locked; we allow
// draft/rejected/approved edits (approved edits re-open review at the service
// layer if desired). Returns ErrNotFound if the seller doesn't own it.
func (r *ProductRepository) UpdateSellerProduct(ctx context.Context, tx pgx.Tx, productID, sellerID string, in UpdateProductInput) error {
	season := in.Season
	if season == "" {
		season = "all"
	}
	ct, err := tx.Exec(ctx, `
		UPDATE products
		SET title = $3, description = $4, brand_id = $5, gender_id = $6,
		    category_id = $7, season = $8
		WHERE id = $1 AND created_by_seller = $2`,
		productID, sellerID, in.Title, in.Description, in.BrandID, in.GenderID, in.CategoryID, season)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateListingPrice updates the seller listing MRP/selling price + discount for
// a product's primary listing.
func (r *ProductRepository) UpdateListingPrice(ctx context.Context, tx pgx.Tx, productID, sellerID string, mrp, price, discount float64) error {
	_, err := tx.Exec(ctx, `
		UPDATE seller_listings
		SET mrp = $3, selling_price = $4, discount_percent = $5
		WHERE product_id = $1 AND seller_id = $2`,
		productID, sellerID, mrp, price, discount)
	if err != nil {
		return err
	}
	// Keep the canonical variant price in sync for the storefront detail view.
	_, err = tx.Exec(ctx, `
		UPDATE product_variants
		SET mrp = $2, selling_price = $3
		WHERE product_id = $1`, productID, mrp, price)
	return err
}

// ReplaceImages deletes all images for a product and inserts the given set.
func (r *ProductRepository) ReplaceImages(ctx context.Context, tx pgx.Tx, productID string, images []ImageInput) error {
	if _, err := tx.Exec(ctx, `DELETE FROM product_images WHERE product_id = $1`, productID); err != nil {
		return err
	}
	for i, img := range images {
		if _, err := tx.Exec(ctx, `
			INSERT INTO product_images (product_id, file_key, alt_text, sort_order, is_primary)
			VALUES ($1,$2,$3,$4,$5)`,
			productID, img.FileKey, img.AltText, i, i == 0); err != nil {
			return err
		}
	}
	return nil
}

// PrimaryListingID returns the seller's primary listing id for a product.
func (r *ProductRepository) PrimaryListingID(ctx context.Context, productID, sellerID string) (string, error) {
	var id string
	err := r.pool.QueryRow(ctx,
		`SELECT id FROM seller_listings WHERE product_id = $1 AND seller_id = $2 ORDER BY created_at LIMIT 1`,
		productID, sellerID).Scan(&id)
	if err == pgx.ErrNoRows {
		return "", ErrNotFound
	}
	return id, err
}

// VerifyListingOwner confirms a listing belongs to the given product + seller.
// Returns ErrNotFound otherwise (prevents cross-seller stock edits / IDOR).
func (r *ProductRepository) VerifyListingOwner(ctx context.Context, listingID, productID, sellerID string) error {
	var ok bool
	if err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM seller_listings WHERE id = $1 AND product_id = $2 AND seller_id = $3)`,
		listingID, productID, sellerID).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	return nil
}

// ImageInput is a single image to attach to a product.
type ImageInput struct {
	FileKey string
	AltText string
}

// ResolveCategoryBySlug returns the id of the category with the given slug,
// creating a gender-neutral top-level category on the fly if it doesn't exist
// yet (so the seller "type" categories always work even before seeding). The
// title-cased slug is used as the display name.
func (r *ProductRepository) ResolveCategoryBySlug(ctx context.Context, slug string) (string, error) {
	slug = strings.TrimSpace(strings.ToLower(slug))
	if slug == "" {
		return "", nil
	}
	var id string
	err := r.pool.QueryRow(ctx, `SELECT id FROM categories WHERE slug = $1`, slug).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != pgx.ErrNoRows {
		return "", err
	}
	// Auto-create as a level-0, gender-neutral category.
	name := strings.ToUpper(slug[:1]) + slug[1:]
	err = r.pool.QueryRow(ctx, `
		INSERT INTO categories (name, slug, level, gender_id)
		VALUES ($1, $2, 0, NULL)
		ON CONFLICT (slug) DO UPDATE SET name = categories.name
		RETURNING id`, name, slug).Scan(&id)
	return id, err
}

// ResolveBrandByName returns the id of the brand with the given (free-text)
// name, creating it on the fly if it doesn't exist. Matching is by slug so
// "Nike" and "nike" map to the same brand.
func (r *ProductRepository) ResolveBrandByName(ctx context.Context, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", nil
	}
	slug := Slugify(name)
	if slug == "" {
		return "", nil
	}
	var id string
	err := r.pool.QueryRow(ctx, `SELECT id FROM brands WHERE slug = $1`, slug).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != pgx.ErrNoRows {
		return "", err
	}
	err = r.pool.QueryRow(ctx, `
		INSERT INTO brands (name, slug)
		VALUES ($1, $2)
		ON CONFLICT (slug) DO UPDATE SET name = brands.name
		RETURNING id`, name, slug).Scan(&id)
	return id, err
}

// SizeAttributeID returns the id of the 'size' attribute, creating it if the
// seed hasn't run yet.
func (r *ProductRepository) SizeAttributeID(ctx context.Context, tx pgx.Tx) (string, error) {
	var id string
	q := `INSERT INTO attributes (code, name, input_type, is_variant, is_filterable, sort_order)
	      VALUES ('size','Size','select',TRUE,TRUE,1)
	      ON CONFLICT (code) DO UPDATE SET name = attributes.name
	      RETURNING id`
	err := tx.QueryRow(ctx, q).Scan(&id)
	return id, err
}

// ResolveSizeValueID returns the attribute_value id for a size label (e.g. "M"
// or "8"), creating it under the 'size' attribute if it doesn't exist. This lets
// sellers use both alpha (S/M/L) and numeric (shoe) sizes freely.
func (r *ProductRepository) ResolveSizeValueID(ctx context.Context, tx pgx.Tx, attrID, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	var id string
	err := tx.QueryRow(ctx,
		`SELECT id FROM attribute_values WHERE attribute_id = $1 AND value = $2`, attrID, value).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != pgx.ErrNoRows {
		return "", err
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO attribute_values (attribute_id, value, display_value)
		VALUES ($1, $2, $2)
		ON CONFLICT (attribute_id, value) DO UPDATE SET value = attribute_values.value
		RETURNING id`, attrID, value).Scan(&id)
	return id, err
}

// CreateVariant inserts a variant for a product.
func (r *ProductRepository) CreateVariant(ctx context.Context, tx pgx.Tx, productID, code string, mrp, price float64, barcode *string, weight int) (string, error) {
	var id string
	q := `INSERT INTO product_variants (product_id, variant_code, barcode, mrp, selling_price, weight_grams)
	      VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`
	var err error
	if tx != nil {
		err = tx.QueryRow(ctx, q, productID, code, barcode, mrp, price, weight).Scan(&id)
	} else {
		err = r.pool.QueryRow(ctx, q, productID, code, barcode, mrp, price, weight).Scan(&id)
	}
	return id, err
}

// SetVariantAttribute links a variant to an attribute value (Size=M etc.).
func (r *ProductRepository) SetVariantAttribute(ctx context.Context, tx pgx.Tx, variantID, attributeID, valueID string) error {
	q := `INSERT INTO product_variant_attributes (variant_id, attribute_id, attribute_value_id)
	      VALUES ($1,$2,$3)
	      ON CONFLICT (variant_id, attribute_id) DO UPDATE SET attribute_value_id = EXCLUDED.attribute_value_id`
	if tx != nil {
		_, err := tx.Exec(ctx, q, variantID, attributeID, valueID)
		return err
	}
	_, err := r.pool.Exec(ctx, q, variantID, attributeID, valueID)
	return err
}

// CreateSellerListing connects a seller to a product/variant.
func (r *ProductRepository) CreateSellerListing(ctx context.Context, tx pgx.Tx, l SellerListing) (string, error) {
	var id string
	q := `INSERT INTO seller_listings
	        (seller_id, product_id, variant_id, sku, mrp, selling_price, discount_percent, currency, status)
	      VALUES ($1,$2,$3,$4,$5,$6,$7,COALESCE(NULLIF($8,''),'INR'),'active')
	      RETURNING id`
	var err error
	if tx != nil {
		err = tx.QueryRow(ctx, q, l.SellerID, l.ProductID, l.VariantID, l.SKU, l.MRP, l.SellingPrice, l.DiscountPercent, l.Currency).Scan(&id)
	} else {
		err = r.pool.QueryRow(ctx, q, l.SellerID, l.ProductID, l.VariantID, l.SKU, l.MRP, l.SellingPrice, l.DiscountPercent, l.Currency).Scan(&id)
	}
	return id, err
}

// AddImage attaches an image record to a product.
func (r *ProductRepository) AddImage(ctx context.Context, tx pgx.Tx, productID, fileKey, alt string, sortOrder int, isPrimary bool) error {
	q := `INSERT INTO product_images (product_id, file_key, alt_text, sort_order, is_primary)
	      VALUES ($1,$2,$3,$4,$5)`
	if tx != nil {
		_, err := tx.Exec(ctx, q, productID, fileKey, alt, sortOrder, isPrimary)
		return err
	}
	_, err := r.pool.Exec(ctx, q, productID, fileKey, alt, sortOrder, isPrimary)
	return err
}

// SubmitForReview moves a draft/rejected product to 'submitted'.
func (r *ProductRepository) SubmitForReview(ctx context.Context, productID, sellerID string) error {
	ct, err := r.pool.Exec(ctx, `
		UPDATE products SET status = 'submitted'
		WHERE id = $1 AND created_by_seller = $2 AND status IN ('draft','rejected')`,
		productID, sellerID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetApprovalStatus is used by admin to approve/reject/suspend a product.
func (r *ProductRepository) SetApprovalStatus(ctx context.Context, productID, status, reason, adminID string) error {
	ct, err := r.pool.Exec(ctx, `
		UPDATE products
		SET status = $2,
		    rejection_reason = CASE WHEN $2 = 'rejected' THEN $3 ELSE '' END,
		    approved_by = CASE WHEN $2 = 'approved' THEN $4::uuid ELSE approved_by END,
		    approved_at = CASE WHEN $2 = 'approved' THEN now() ELSE approved_at END,
		    published_at = CASE WHEN $2 = 'approved' THEN now() ELSE published_at END
		WHERE id = $1`,
		productID, status, reason, adminID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetProduct returns a product with images and variants (admin/internal use).
func (r *ProductRepository) GetProduct(ctx context.Context, id string) (*Product, error) {
	p, err := r.scanProduct(ctx, `WHERE p.id = $1`, id)
	if err != nil {
		return nil, err
	}
	return r.hydrate(ctx, p, "")
}

// GetProductForSeller returns a product only if it was created by the given
// seller. Prevents a seller reading another seller's product by ID (IDOR).
func (r *ProductRepository) GetProductForSeller(ctx context.Context, id, sellerID string) (*Product, error) {
	var ok bool
	if err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM products WHERE id = $1 AND created_by_seller = $2)`,
		id, sellerID).Scan(&ok); err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}
	p, err := r.scanProduct(ctx, `WHERE p.id = $1`, id)
	if err != nil {
		return nil, err
	}
	if _, err := r.hydrate(ctx, p, sellerID); err != nil {
		return nil, err
	}
	// Total available stock across the seller's listings (all sizes summed).
	total := 0
	for _, v := range p.Variants {
		total += v.AvailableQty
	}
	p.AvailableQty = &total
	return p, nil
}

// GetProductBySlug returns an approved product by slug (storefront).
func (r *ProductRepository) GetProductBySlug(ctx context.Context, slug string) (*Product, error) {
	p, err := r.scanProduct(ctx, `WHERE p.slug = $1 AND p.status = 'approved'`, slug)
	if err != nil {
		return nil, err
	}
	if _, err := r.hydrate(ctx, p, ""); err != nil {
		return nil, err
	}
	// Total available stock across variants (each variant's active listing),
	// so the storefront can show an out-of-stock state on the product page.
	total := 0
	for _, v := range p.Variants {
		total += v.AvailableQty
	}
	p.AvailableQty = &total
	return p, nil
}

func (r *ProductRepository) scanProduct(ctx context.Context, where string, arg any) (*Product, error) {
	var p Product
	q := `SELECT p.id, p.title, p.slug, p.description, p.brand_id, coalesce(b.name,''),
	             p.gender_id, p.category_id, p.subcategory_id, p.status, p.rejection_reason,
	             p.rating_avg, p.rating_count, p.hsn_code, p.tax_percent, p.season,
	             p.created_by_seller, p.created_at
	      FROM products p
	      LEFT JOIN brands b ON b.id = p.brand_id ` + where
	err := r.pool.QueryRow(ctx, q, arg).Scan(
		&p.ID, &p.Title, &p.Slug, &p.Description, &p.BrandID, &p.BrandName,
		&p.GenderID, &p.CategoryID, &p.SubcategoryID, &p.Status, &p.RejectionReason,
		&p.RatingAvg, &p.RatingCount, &p.HSNCode, &p.TaxPercent, &p.Season,
		&p.CreatedBySeller, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// hydrate loads a product's images and variants. When sellerID is non-empty the
// variants' listing id + stock come from that seller's listings; otherwise the
// storefront's active listing per variant is used.
func (r *ProductRepository) hydrate(ctx context.Context, p *Product, sellerID string) (*Product, error) {
	// Images.
	irows, err := r.pool.Query(ctx, `
		SELECT id, file_key, alt_text, sort_order, is_primary
		FROM product_images WHERE product_id = $1 ORDER BY is_primary DESC, sort_order`, p.ID)
	if err != nil {
		return nil, err
	}
	defer irows.Close()
	for irows.Next() {
		var img Image
		if err := irows.Scan(&img.ID, &img.FileKey, &img.AltText, &img.SortOrder, &img.IsPrimary); err != nil {
			return nil, err
		}
		p.Images = append(p.Images, img)
	}

	// Variants with their size label + this seller's (or the active) listing +
	// live available stock. The listing join is scoped by seller when given.
	listingJoin := `LEFT JOIN LATERAL (
		SELECT sl.id, inv.available_quantity
		FROM seller_listings sl
		LEFT JOIN inventory inv ON inv.seller_listing_id = sl.id
		WHERE sl.variant_id = pv.id ` + `AND ($2 = '' OR sl.seller_id = $2::uuid) ` +
		`AND ($2 <> '' OR sl.status = 'active')
		ORDER BY sl.created_at LIMIT 1
	) sl ON TRUE`
	vrows, err := r.pool.Query(ctx, `
		SELECT pv.id, pv.product_id, pv.variant_code, pv.barcode, pv.mrp, pv.selling_price,
		       pv.weight_grams, pv.is_active,
		       COALESCE(av.value, '') AS size,
		       COALESCE(sl.id::text, '') AS seller_listing_id,
		       COALESCE(sl.available_quantity, 0) AS available_qty
		FROM product_variants pv
		LEFT JOIN product_variant_attributes pva
		       ON pva.variant_id = pv.id
		      AND pva.attribute_id = (SELECT id FROM attributes WHERE code = 'size')
		LEFT JOIN attribute_values av ON av.id = pva.attribute_value_id
		`+listingJoin+`
		WHERE pv.product_id = $1
		ORDER BY av.sort_order NULLS LAST, pv.variant_code`, p.ID, sellerID)
	if err != nil {
		return nil, err
	}
	defer vrows.Close()
	for vrows.Next() {
		var v Variant
		if err := vrows.Scan(&v.ID, &v.ProductID, &v.VariantCode, &v.Barcode,
			&v.MRP, &v.SellingPrice, &v.WeightGrams, &v.IsActive,
			&v.Size, &v.SellerListingID, &v.AvailableQty); err != nil {
			return nil, err
		}
		p.Variants = append(p.Variants, v)
	}
	return p, vrows.Err()
}

// ListSellerProducts returns products created by a seller, optionally filtered
// by status, with pagination.
func (r *ProductRepository) ListSellerProducts(ctx context.Context, sellerID, status string, limit, offset int) ([]Product, int64, error) {
	args := []any{sellerID}
	where := `WHERE p.created_by_seller = $1`
	if status != "" {
		args = append(args, status)
		where += ` AND p.status = $2`
	}

	var total int64
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM products p `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, limit, offset)
	// LATERAL join pulls the seller's own listing price for each product so the
	// list view can show the final selling price without hydrating variants.
	q := `SELECT p.id, p.title, p.slug, p.description, p.brand_id, coalesce(b.name,''),
	             p.gender_id, p.category_id, p.subcategory_id, p.status, p.rejection_reason,
	             p.rating_avg, p.rating_count, p.hsn_code, p.tax_percent, p.season,
	             p.created_by_seller, p.created_at, sl.selling_price, sl.mrp, sl.available_quantity,
	             coalesce((SELECT file_key FROM product_images WHERE product_id = p.id ORDER BY is_primary DESC, sort_order LIMIT 1),'')
	      FROM products p
	      LEFT JOIN brands b ON b.id = p.brand_id
	      LEFT JOIN LATERAL (
	          SELECT sl.selling_price, sl.mrp, inv.available_quantity
	          FROM seller_listings sl
	          LEFT JOIN inventory inv ON inv.seller_listing_id = sl.id
	          WHERE sl.product_id = p.id AND sl.seller_id = $1
	          ORDER BY sl.created_at LIMIT 1
	      ) sl ON TRUE ` + where +
		` ORDER BY p.created_at DESC LIMIT $` + itoa(len(args)-1) + ` OFFSET $` + itoa(len(args))
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []Product
	for rows.Next() {
		var p Product
		var price, mrp *float64
		var qty *int
		var imgKey string
		if err := rows.Scan(&p.ID, &p.Title, &p.Slug, &p.Description, &p.BrandID, &p.BrandName,
			&p.GenderID, &p.CategoryID, &p.SubcategoryID, &p.Status, &p.RejectionReason,
			&p.RatingAvg, &p.RatingCount, &p.HSNCode, &p.TaxPercent, &p.Season,
			&p.CreatedBySeller, &p.CreatedAt, &price, &mrp, &qty, &imgKey); err != nil {
			return nil, 0, err
		}
		p.SellingPrice = price
		p.MRP = mrp
		p.AvailableQty = qty
		if imgKey != "" {
			p.Images = []Image{{FileKey: imgKey, IsPrimary: true}}
		}
		out = append(out, p)
	}
	return out, total, rows.Err()
}

// ListForAdmin lists products for admin review, filtered by status.
func (r *ProductRepository) ListForAdmin(ctx context.Context, status string, limit, offset int) ([]Product, int64, error) {
	args := []any{}
	where := ``
	if status != "" {
		args = append(args, status)
		where = `WHERE p.status = $1`
	}
	var total int64
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM products p `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, limit, offset)
	q := `SELECT p.id, p.title, p.slug, p.description, p.brand_id, coalesce(b.name,''),
	             p.gender_id, p.category_id, p.subcategory_id, p.status, p.rejection_reason,
	             p.rating_avg, p.rating_count, p.hsn_code, p.tax_percent, p.season,
	             p.created_by_seller, p.created_at
	      FROM products p LEFT JOIN brands b ON b.id = p.brand_id ` + where +
		` ORDER BY p.created_at DESC LIMIT $` + itoa(len(args)-1) + ` OFFSET $` + itoa(len(args))
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []Product
	for rows.Next() {
		var p Product
		if err := rows.Scan(&p.ID, &p.Title, &p.Slug, &p.Description, &p.BrandID, &p.BrandName,
			&p.GenderID, &p.CategoryID, &p.SubcategoryID, &p.Status, &p.RejectionReason,
			&p.RatingAvg, &p.RatingCount, &p.HSNCode, &p.TaxPercent, &p.Season,
			&p.CreatedBySeller, &p.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, p)
	}
	return out, total, rows.Err()
}

// itoa is a tiny helper to build positional parameter placeholders.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		return "-" + string(digits)
	}
	return string(digits)
}
