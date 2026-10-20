package coupons

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/carlitos/backend/internal/httpx"
)

// Service holds coupon business logic (validation + discount computation).
type Service struct {
	repo *Repository
}

// NewService builds a coupon Service.
func NewService(repo *Repository) *Service { return &Service{repo: repo} }

// Repo exposes the repository (used by the handler for admin reads/writes).
func (s *Service) Repo() *Repository { return s.repo }

// Quote is the result of validating a coupon against a cart subtotal.
type Quote struct {
	Code     string  `json:"code"`
	Discount float64 `json:"discount"`
	Message  string  `json:"message"`
}

// round2 rounds to 2 decimals (paise-accurate).
func round2(v float64) float64 { return float64(int64(v*100+0.5)) / 100.0 }

// computeDiscount applies the coupon's type/value/cap to a subtotal.
func computeDiscount(c *Coupon, subtotal float64) float64 {
	var d float64
	if c.DiscountType == "percent" {
		d = subtotal * c.DiscountValue / 100.0
	} else {
		d = c.DiscountValue
	}
	if c.MaxDiscount != nil && *c.MaxDiscount > 0 && d > *c.MaxDiscount {
		d = *c.MaxDiscount
	}
	if d > subtotal {
		d = subtotal // never discount below zero
	}
	return round2(d)
}

// validate runs all eligibility checks and returns the coupon + discount, or a
// user-facing error. customerID may be empty for a generic (pre-login) preview;
// per-user limits are only enforced when a customerID is supplied.
func (s *Service) validate(ctx context.Context, code string, subtotal float64, customerID string) (*Coupon, float64, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, 0, httpx.BadRequest("coupon_required", "Enter a coupon code")
	}
	c, err := s.repo.GetByCode(ctx, code)
	if err != nil {
		return nil, 0, httpx.BadRequest("invalid_coupon", "This coupon code is not valid")
	}
	if !c.IsActive {
		return nil, 0, httpx.BadRequest("coupon_inactive", "This coupon is no longer active")
	}
	now := time.Now()
	if c.StartsAt != nil && now.Before(*c.StartsAt) {
		return nil, 0, httpx.BadRequest("coupon_not_started", "This coupon is not active yet")
	}
	if c.EndsAt != nil && now.After(*c.EndsAt) {
		return nil, 0, httpx.BadRequest("coupon_expired", "This coupon has expired")
	}
	if subtotal < c.MinOrderAmount {
		return nil, 0, httpx.BadRequest("coupon_min_order",
			"Add items worth more to use this coupon")
	}
	if c.UsageLimitTotal != nil && c.UsedCount >= *c.UsageLimitTotal {
		return nil, 0, httpx.BadRequest("coupon_exhausted", "This coupon has reached its usage limit")
	}
	if customerID != "" && c.UsageLimitPerUser != nil {
		used, err := s.repo.PerUserCount(ctx, c.ID, customerID)
		if err == nil && used >= *c.UsageLimitPerUser {
			return nil, 0, httpx.BadRequest("coupon_user_limit", "You have already used this coupon")
		}
	}
	d := computeDiscount(c, subtotal)
	if d <= 0 {
		return nil, 0, httpx.BadRequest("coupon_no_discount", "This coupon gives no discount on your cart")
	}
	return c, d, nil
}

// Preview validates a coupon against a subtotal (and optional customer) and
// returns the discount it would give — used by the checkout UI before placing
// the order. It does NOT consume the coupon.
func (s *Service) Preview(ctx context.Context, code string, subtotal float64, customerID string) (*Quote, error) {
	c, d, err := s.validate(ctx, code, subtotal, customerID)
	if err != nil {
		return nil, err
	}
	return &Quote{Code: c.Code, Discount: d, Message: discountLabel(c)}, nil
}

// ValidateForCheckout re-validates a coupon at order time and returns the
// coupon + discount. Called by the orders module just before it writes the
// order; the actual redemption is recorded via RedeemTx inside the same tx.
func (s *Service) ValidateForCheckout(ctx context.Context, code string, subtotal float64, customerID string) (couponID, normalizedCode string, discount float64, err error) {
	c, d, verr := s.validate(ctx, code, subtotal, customerID)
	if verr != nil {
		return "", "", 0, verr
	}
	return c.ID, c.Code, d, nil
}

// RedeemTx records a redemption and increments used_count inside an existing
// transaction (the checkout tx). Keeping it in the same tx means the coupon is
// only consumed if the order commits.
func (s *Service) RedeemTx(ctx context.Context, tx pgx.Tx, couponID, code, customerID, orderID string, discount float64) error {
	if _, err := tx.Exec(ctx,
		`INSERT INTO coupon_redemptions (coupon_id, coupon_code, customer_id, order_id, discount_amount)
		 VALUES ($1,$2,$3,$4,$5)`, couponID, code, customerID, orderID, discount); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `UPDATE coupons SET used_count = used_count + 1 WHERE id=$1`, couponID)
	return err
}

func discountLabel(c *Coupon) string {
	if c.DiscountType == "percent" {
		return "Coupon applied"
	}
	return "Coupon applied"
}
