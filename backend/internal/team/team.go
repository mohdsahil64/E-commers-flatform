// Package team implements the company "Team & Management" roster. Members are
// seeded with system-default role/bio/sort_order; admins only edit each
// member's name and photo. The storefront shows the active roster ordered by
// sort_order then name.
package team

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/carlitos/backend/internal/httpx"
)

// TeamMember view model. Mirrors the team_members table columns.
type TeamMember struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	Bio       string `json:"bio"`
	PhotoKey  string `json:"photo_key"`
	SortOrder int    `json:"sort_order"`
	IsActive  bool   `json:"is_active"`
}

// Repository provides team persistence.
type Repository struct{ pool *pgxpool.Pool }

// NewRepository builds a team Repository.
func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// ListActive returns the active roster for the storefront, ordered by
// sort_order then name.
func (r *Repository) ListActive(ctx context.Context) ([]TeamMember, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, role, bio, coalesce(photo_key,''), sort_order, is_active
		FROM team_members
		WHERE is_active = TRUE
		ORDER BY sort_order, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TeamMember
	for rows.Next() {
		var m TeamMember
		if err := rows.Scan(&m.ID, &m.Name, &m.Role, &m.Bio, &m.PhotoKey,
			&m.SortOrder, &m.IsActive); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// AdminList returns every member (active or not) for the admin manager,
// ordered by sort_order then name.
func (r *Repository) AdminList(ctx context.Context) ([]TeamMember, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, role, bio, coalesce(photo_key,''), sort_order, is_active
		FROM team_members
		ORDER BY sort_order, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TeamMember
	for rows.Next() {
		var m TeamMember
		if err := rows.Scan(&m.ID, &m.Name, &m.Role, &m.Bio, &m.PhotoKey,
			&m.SortOrder, &m.IsActive); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// UpdateMember updates only the editable fields: name and photo. An empty
// photoKey keeps the existing photo (admin chose not to re-upload), mirroring
// how banners preserve their image on edit. role/bio/sort_order/is_active are
// system defaults and never writable here.
func (r *Repository) UpdateMember(ctx context.Context, id, name, photoKey string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE team_members SET
			name = $2,
			photo_key = COALESCE(NULLIF($3,''), photo_key),
			updated_at = now()
		WHERE id = $1`, id, name, photoKey)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return httpx.NotFound("team_member_not_found", "Team member not found")
	}
	return nil
}
