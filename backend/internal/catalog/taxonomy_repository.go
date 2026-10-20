package catalog

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TaxonomyRepository provides access to genders, categories, brands and attributes.
type TaxonomyRepository struct {
	pool *pgxpool.Pool
}

// NewTaxonomyRepository builds a TaxonomyRepository.
func NewTaxonomyRepository(pool *pgxpool.Pool) *TaxonomyRepository {
	return &TaxonomyRepository{pool: pool}
}

// ListGenders returns active genders ordered for navigation.
func (r *TaxonomyRepository) ListGenders(ctx context.Context) ([]Gender, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, code, name, sort_order, is_active
		FROM genders WHERE is_active = TRUE ORDER BY sort_order, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Gender
	for rows.Next() {
		var g Gender
		if err := rows.Scan(&g.ID, &g.Code, &g.Name, &g.SortOrder, &g.IsActive); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// ListCategories returns all active categories (flat). Callers can build a tree.
func (r *TaxonomyRepository) ListCategories(ctx context.Context) ([]Category, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, parent_id, gender_id, name, slug, level, description,
		       image_key, sort_order, is_active
		FROM categories WHERE is_active = TRUE
		ORDER BY level, sort_order, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Category
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.ParentID, &c.GenderID, &c.Name, &c.Slug,
			&c.Level, &c.Description, &c.ImageKey, &c.SortOrder, &c.IsActive); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// BuildTree assembles a nested category tree from a flat list.
func BuildTree(flat []Category) []Category {
	byID := map[string]*Category{}
	for i := range flat {
		flat[i].Children = []Category{}
		byID[flat[i].ID] = &flat[i]
	}
	var roots []Category
	for i := range flat {
		c := &flat[i]
		if c.ParentID != nil {
			if parent, ok := byID[*c.ParentID]; ok {
				parent.Children = append(parent.Children, *c)
				continue
			}
		}
		roots = append(roots, *c)
	}
	return roots
}

// GetCategoryBySlug returns a single category by its slug.
func (r *TaxonomyRepository) GetCategoryBySlug(ctx context.Context, slug string) (*Category, error) {
	var c Category
	err := r.pool.QueryRow(ctx, `
		SELECT id, parent_id, gender_id, name, slug, level, description,
		       image_key, sort_order, is_active
		FROM categories WHERE slug = $1 AND is_active = TRUE`, slug).
		Scan(&c.ID, &c.ParentID, &c.GenderID, &c.Name, &c.Slug, &c.Level,
			&c.Description, &c.ImageKey, &c.SortOrder, &c.IsActive)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// CreateCategory inserts a category (admin).
func (r *TaxonomyRepository) CreateCategory(ctx context.Context, c Category) (string, error) {
	var id string
	err := r.pool.QueryRow(ctx, `
		INSERT INTO categories (parent_id, gender_id, name, slug, level, description, sort_order, is_active)
		VALUES ($1,$2,$3,$4,$5,$6,$7,COALESCE($8,TRUE))
		RETURNING id`,
		c.ParentID, c.GenderID, c.Name, c.Slug, c.Level, c.Description, c.SortOrder, c.IsActive).Scan(&id)
	return id, err
}

// ListBrands returns active brands.
func (r *TaxonomyRepository) ListBrands(ctx context.Context) ([]Brand, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, slug, logo_key, is_active
		FROM brands WHERE is_active = TRUE ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Brand
	for rows.Next() {
		var b Brand
		if err := rows.Scan(&b.ID, &b.Name, &b.Slug, &b.LogoKey, &b.IsActive); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// ListAttributes returns attributes with their values.
func (r *TaxonomyRepository) ListAttributes(ctx context.Context) ([]Attribute, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, code, name, input_type, is_variant, is_filterable, sort_order
		FROM attributes ORDER BY sort_order, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	attrs := []Attribute{}
	index := map[string]int{}
	for rows.Next() {
		var a Attribute
		if err := rows.Scan(&a.ID, &a.Code, &a.Name, &a.InputType, &a.IsVariant,
			&a.IsFilterable, &a.SortOrder); err != nil {
			return nil, err
		}
		a.Values = []AttributeValue{}
		index[a.ID] = len(attrs)
		attrs = append(attrs, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	vrows, err := r.pool.Query(ctx, `
		SELECT id, attribute_id, value, display_value, meta, sort_order
		FROM attribute_values ORDER BY sort_order, value`)
	if err != nil {
		return nil, err
	}
	defer vrows.Close()
	for vrows.Next() {
		var v AttributeValue
		if err := vrows.Scan(&v.ID, &v.AttributeID, &v.Value, &v.DisplayValue, &v.Meta, &v.SortOrder); err != nil {
			return nil, err
		}
		if i, ok := index[v.AttributeID]; ok {
			attrs[i].Values = append(attrs[i].Values, v)
		}
	}
	return attrs, vrows.Err()
}
