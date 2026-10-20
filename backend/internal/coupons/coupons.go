// Package coupons implements discount coupons: admin management (create/update,
// usage limits, min cart, validity window), customer validation/preview, and
// server-side application during checkout. All money math is done on the server;
// the frontend never computes or is trusted for the discount.
package coupons

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/carlitos/backend/internal/httpx"
)

// Coupon is the admin/customer view model for a coupon.
type Coupon struct {
	ID                string     `json:"id"`
	Code              string     `json:"code"`
	Description       string     `json:"description"`
	DiscountType      string     `json:"discount_type"` // percent | fixed
	DiscountValue     float64    `json:"discount_value"`
	MaxDiscount       *float64   `json:"max_discount"`
	MinOrderAmount    float64    `json:"min_order_amount"`
	UsageLimitTotal   *int       `json:"usage_limit_total"`
	UsageLimitPerUser *int       `json:"usage_limit_per_user"`
	UsedCount         int        `json:"used_count"`
	StartsAt          *time.Time `json:"starts_at"`
	EndsAt            *time.Time `json:"ends_at"`
	IsActive          bool       `json:"is_active"`
	CreatedAt         time.Time  `json:"created_at"`
}

// Redemption is a single coupon use (admin history view).
type Redemption struct {
	ID             string    `json:"id"`
	CouponCode     string    `json:"coupon_code"`
	CustomerID     string    `json:"customer_id"`
	CustomerName   string    `json:"customer_name"`
	OrderID        *string   `json:"order_id"`
	OrderNumber    string    `json:"order_number"`
	DiscountAmount float64   `json:"discount_amount"`
	CreatedAt      time.Time `json:"created_at"`
}

// Repository provides coupon persistence.
type Repository struct{ pool *pgxpool.Pool }

// NewRepository builds a coupon Repository.
func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// Pool exposes the pool so the orders module can redeem inside its checkout tx.
func (r *Repository) Pool() *pgxpool.Pool { return r.pool }

const couponCols = `id, code, description, discount_type, discount_value, max_discount,
	min_order_amount, usage_limit_total, usage_limit_per_user, used_count,
	starts_at, ends_at, is_active, created_at`

func scanCoupon(row pgx.Row) (*Coupon, error) {
	var c Coupon
	if err := row.Scan(&c.ID, &c.Code, &c.Description, &c.DiscountType, &c.DiscountValue,
		&c.MaxDiscount, &c.MinOrderAmount, &c.UsageLimitTotal, &c.UsageLimitPerUser,
		&c.UsedCount, &c.StartsAt, &c.EndsAt, &c.IsActive, &c.CreatedAt); err != nil {
		return nil, err
	}
	return &c, nil
}

// List returns all coupons (admin), newest first.
func (r *Repository) List(ctx context.Context) ([]Coupon, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+couponCols+` FROM coupons ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Coupon
	for rows.Next() {
		c, err := scanCoupon(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

// ListAvailable returns active, currently-valid coupons that still have
// redemptions left — the "offers" a customer can see and potentially use. It
// orders by the minimum order amount ascending so the easiest-to-unlock offers
// come first (handy for the storefront's "unlock progress" display).
func (r *Repository) ListAvailable(ctx context.Context) ([]Coupon, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+couponCols+` FROM coupons
		WHERE is_active = TRUE
		  AND (starts_at IS NULL OR starts_at <= now())
		  AND (ends_at   IS NULL OR ends_at   >= now())
		  AND (usage_limit_total IS NULL OR used_count < usage_limit_total)
		ORDER BY min_order_amount ASC, created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Coupon
	for rows.Next() {
		c, err := scanCoupon(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

// GetByCode returns a coupon by its (case-insensitive) code.
func (r *Repository) GetByCode(ctx context.Context, code string) (*Coupon, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+couponCols+` FROM coupons WHERE upper(code)=upper($1)`, code)
	c, err := scanCoupon(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("coupon_not_found", "Coupon not found")
	}
	return c, err
}

// GetByID returns a coupon by id.
func (r *Repository) GetByID(ctx context.Context, id string) (*Coupon, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+couponCols+` FROM coupons WHERE id=$1`, id)
	c, err := scanCoupon(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("coupon_not_found", "Coupon not found")
	}
	return c, err
}

// CreateInput is the admin create/update payload.
type CreateInput struct {
	Code              string   `json:"code"`
	Description       string   `json:"description"`
	DiscountType      string   `json:"discount_type"`
	DiscountValue     float64  `json:"discount_value"`
	MaxDiscount       *float64 `json:"max_discount"`
	MinOrderAmount    float64  `json:"min_order_amount"`
	UsageLimitTotal   *int     `json:"usage_limit_total"`
	UsageLimitPerUser *int     `json:"usage_limit_per_user"`
	StartsAt          *string  `json:"starts_at"`
	EndsAt            *string  `json:"ends_at"`
	IsActive          *bool    `json:"is_active"`
}

func parseTime(s *string) (*time.Time, error) {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil, nil
	}
	// Accept both full RFC3339 and simple date / datetime-local values.
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04", "2006-01-02"} {
		if t, err := time.Parse(layout, strings.TrimSpace(*s)); err == nil {
			return &t, nil
		}
	}
	return nil, errors.New("invalid date")
}

// Create inserts a new coupon.
func (r *Repository) Create(ctx context.Context, in CreateInput) (string, error) {
	starts, err := parseTime(in.StartsAt)
	if err != nil {
		return "", httpx.BadRequest("invalid_date", "Invalid start date")
	}
	ends, err := parseTime(in.EndsAt)
	if err != nil {
		return "", httpx.BadRequest("invalid_date", "Invalid end date")
	}
	active := true
	if in.IsActive != nil {
		active = *in.IsActive
	}
	var id string
	err = r.pool.QueryRow(ctx, `
		INSERT INTO coupons (code, description, discount_type, discount_value, max_discount,
			min_order_amount, usage_limit_total, usage_limit_per_user, starts_at, ends_at, is_active)
		VALUES (upper($1),$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id`,
		strings.TrimSpace(in.Code), in.Description, in.DiscountType, in.DiscountValue, in.MaxDiscount,
		in.MinOrderAmount, in.UsageLimitTotal, in.UsageLimitPerUser, starts, ends, active).Scan(&id)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return "", httpx.Conflict("coupon_exists", "A coupon with this code already exists")
		}
		return "", err
	}
	return id, nil
}

// Update modifies an existing coupon.
func (r *Repository) Update(ctx context.Context, id string, in CreateInput) error {
	starts, err := parseTime(in.StartsAt)
	if err != nil {
		return httpx.BadRequest("invalid_date", "Invalid start date")
	}
	ends, err := parseTime(in.EndsAt)
	if err != nil {
		return httpx.BadRequest("invalid_date", "Invalid end date")
	}
	active := true
	if in.IsActive != nil {
		active = *in.IsActive
	}
	ct, err := r.pool.Exec(ctx, `
		UPDATE coupons SET description=$2, discount_type=$3, discount_value=$4, max_discount=$5,
			min_order_amount=$6, usage_limit_total=$7, usage_limit_per_user=$8,
			starts_at=$9, ends_at=$10, is_active=$11
		WHERE id=$1`,
		id, in.Description, in.DiscountType, in.DiscountValue, in.MaxDiscount,
		in.MinOrderAmount, in.UsageLimitTotal, in.UsageLimitPerUser, starts, ends, active)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return httpx.NotFound("coupon_not_found", "Coupon not found")
	}
	return nil
}

// SetActive toggles a coupon's active flag.
func (r *Repository) SetActive(ctx context.Context, id string, active bool) error {
	ct, err := r.pool.Exec(ctx, `UPDATE coupons SET is_active=$2 WHERE id=$1`, id, active)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return httpx.NotFound("coupon_not_found", "Coupon not found")
	}
	return nil
}

// Delete removes a coupon.
func (r *Repository) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM coupons WHERE id=$1`, id)
	return err
}

// Redemptions returns usage history (admin), optionally for one coupon.
func (r *Repository) Redemptions(ctx context.Context, couponID string) ([]Redemption, error) {
	q := `
		SELECT cr.id, cr.coupon_code, cr.customer_id, coalesce(c.display_name,''),
		       cr.order_id, coalesce(o.order_number,''), cr.discount_amount, cr.created_at
		FROM coupon_redemptions cr
		LEFT JOIN customers c ON c.id = cr.customer_id
		LEFT JOIN orders o ON o.id = cr.order_id`
	args := []any{}
	if couponID != "" {
		q += ` WHERE cr.coupon_id = $1`
		args = append(args, couponID)
	}
	q += ` ORDER BY cr.created_at DESC LIMIT 500`
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Redemption
	for rows.Next() {
		var rd Redemption
		if err := rows.Scan(&rd.ID, &rd.CouponCode, &rd.CustomerID, &rd.CustomerName,
			&rd.OrderID, &rd.OrderNumber, &rd.DiscountAmount, &rd.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, rd)
	}
	return out, rows.Err()
}

// PerUserCount returns how many times a customer has already redeemed a coupon.
func (r *Repository) PerUserCount(ctx context.Context, couponID, customerID string) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM coupon_redemptions WHERE coupon_id=$1 AND customer_id=$2`,
		couponID, customerID).Scan(&n)
	return n, err
}
