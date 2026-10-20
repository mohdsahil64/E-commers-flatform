package merchandising

import (
	"context"

	"github.com/carlitos/backend/internal/httpx"
)

// Banner view model with resolved targeting.
type Banner struct {
	ID              string            `json:"id"`
	Title           string            `json:"title"`
	Subtitle        string            `json:"subtitle"`
	ImageKey        string            `json:"image_key"`
	MobileImageKey  string            `json:"mobile_image_key"`
	Placement       string            `json:"placement"`
	DestinationType string            `json:"destination_type"`
	Targets         map[string]string `json:"targets"`
	SortOrder       int               `json:"sort_order"`
}

// ListBanners returns active banners for a placement, each with its targeting
// rules resolved into a map so the storefront can build a listing query.
func (r *Repository) ListBanners(ctx context.Context, placement string) ([]Banner, error) {
	args := []any{}
	where := `WHERE is_active = TRUE
	          AND (starts_at IS NULL OR starts_at <= now())
	          AND (ends_at IS NULL OR ends_at >= now())`
	if placement != "" {
		args = append(args, placement)
		where += ` AND placement = $1`
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, title, subtitle, image_key, coalesce(mobile_image_key,''),
		       placement, destination_type, sort_order
		FROM banners `+where+` ORDER BY sort_order`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var banners []Banner
	ids := []string{}
	index := map[string]int{}
	for rows.Next() {
		var b Banner
		if err := rows.Scan(&b.ID, &b.Title, &b.Subtitle, &b.ImageKey, &b.MobileImageKey,
			&b.Placement, &b.DestinationType, &b.SortOrder); err != nil {
			return nil, err
		}
		b.Targets = map[string]string{}
		index[b.ID] = len(banners)
		banners = append(banners, b)
		ids = append(ids, b.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return banners, nil
	}

	// Load targets for all banners in one query.
	trows, err := r.pool.Query(ctx,
		`SELECT banner_id, target_key, target_value FROM banner_targets WHERE banner_id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer trows.Close()
	for trows.Next() {
		var bid, k, v string
		if err := trows.Scan(&bid, &k, &v); err != nil {
			return nil, err
		}
		if i, ok := index[bid]; ok {
			banners[i].Targets[k] = v
		}
	}
	return banners, trows.Err()
}

// AdminListBanners returns every banner (active or not), newest first, with its
// targets resolved — used by the admin manager so hidden/expired banners remain
// editable, unlike the storefront ListBanners which only returns live ones.
func (r *Repository) AdminListBanners(ctx context.Context) ([]Banner, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, title, subtitle, image_key, coalesce(mobile_image_key,''),
		       placement, destination_type, sort_order
		FROM banners ORDER BY sort_order, created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var banners []Banner
	ids := []string{}
	index := map[string]int{}
	for rows.Next() {
		var b Banner
		if err := rows.Scan(&b.ID, &b.Title, &b.Subtitle, &b.ImageKey, &b.MobileImageKey,
			&b.Placement, &b.DestinationType, &b.SortOrder); err != nil {
			return nil, err
		}
		b.Targets = map[string]string{}
		index[b.ID] = len(banners)
		banners = append(banners, b)
		ids = append(ids, b.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return banners, nil
	}
	trows, err := r.pool.Query(ctx,
		`SELECT banner_id, target_key, target_value FROM banner_targets WHERE banner_id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer trows.Close()
	for trows.Next() {
		var bid, k, v string
		if err := trows.Scan(&bid, &k, &v); err != nil {
			return nil, err
		}
		if i, ok := index[bid]; ok {
			banners[i].Targets[k] = v
		}
	}
	return banners, trows.Err()
}

// UpdateBanner updates an existing banner's editable fields and replaces its
// targets. Only the image is optional — when image_key is empty the existing
// image is preserved (admin chose not to re-upload).
func (r *Repository) UpdateBanner(ctx context.Context, b Banner) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE banners SET
			title = $2,
			subtitle = $3,
			image_key = COALESCE(NULLIF($4,''), image_key),
			mobile_image_key = COALESCE(NULLIF($5,''), mobile_image_key),
			placement = COALESCE(NULLIF($6,''), placement),
			destination_type = COALESCE(NULLIF($7,''), destination_type),
			sort_order = $8
		WHERE id = $1`,
		b.ID, b.Title, b.Subtitle, b.ImageKey, b.MobileImageKey,
		b.Placement, b.DestinationType, b.SortOrder)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return httpx.NotFound("banner_not_found", "Banner not found")
	}
	// Replace targets wholesale so removed keys (e.g. cleared cta_label) vanish.
	if _, err := r.pool.Exec(ctx, `DELETE FROM banner_targets WHERE banner_id = $1`, b.ID); err != nil {
		return err
	}
	for k, v := range b.Targets {
		if _, err := r.pool.Exec(ctx,
			`INSERT INTO banner_targets (banner_id, target_key, target_value) VALUES ($1,$2,$3)`,
			b.ID, k, v); err != nil {
			return err
		}
	}
	return nil
}

// CreateBanner inserts a banner with its targets (admin).
func (r *Repository) CreateBanner(ctx context.Context, b Banner) (string, error) {
	var id string
	if err := r.pool.QueryRow(ctx, `
		INSERT INTO banners (title, subtitle, image_key, mobile_image_key, placement, destination_type, sort_order, is_active)
		VALUES ($1,$2,$3,NULLIF($4,''),COALESCE(NULLIF($5,''),'home_hero'),COALESCE(NULLIF($6,''),'category'),$7,TRUE)
		RETURNING id`,
		b.Title, b.Subtitle, b.ImageKey, b.MobileImageKey, b.Placement, b.DestinationType, b.SortOrder).
		Scan(&id); err != nil {
		return "", err
	}
	for k, v := range b.Targets {
		if _, err := r.pool.Exec(ctx,
			`INSERT INTO banner_targets (banner_id, target_key, target_value) VALUES ($1,$2,$3)`,
			id, k, v); err != nil {
			return "", err
		}
	}
	return id, nil
}
