package customers

// Admin-side customer management: list, details, block / unblock.

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/carlitos/backend/internal/httpx"
)

// BlockReason is one selectable reason in the admin "Block customer" dropdown.
type BlockReason struct {
	Code  string `json:"code"`
	Label string `json:"label"`
	// NoteRequired is true when the admin must type a short explanation.
	NoteRequired bool `json:"note_required"`
}

// BlockReasons is the authoritative list of block reasons offered to admins.
// The label is what the customer sees when they try to log in.
var BlockReasons = []BlockReason{
	{Code: "fraud", Label: "Suspected fraudulent activity"},
	{Code: "payment_abuse", Label: "Payment fraud or repeated chargebacks"},
	{Code: "return_abuse", Label: "Return or refund abuse"},
	{Code: "fake_orders", Label: "Fake or prank orders"},
	{Code: "abusive_behavior", Label: "Abusive behaviour towards sellers or staff"},
	{Code: "policy_violation", Label: "Violation of terms and policies"},
	{Code: "duplicate_account", Label: "Duplicate or multiple accounts"},
	{Code: "other", Label: "Other", NoteRequired: true},
}

func findBlockReason(code string) (BlockReason, bool) {
	for _, r := range BlockReasons {
		if r.Code == code {
			return r, true
		}
	}
	return BlockReason{}, false
}

// AdminCustomer is one row in the admin customer list.
type AdminCustomer struct {
	ID          string     `json:"id"`
	UserID      string     `json:"user_id"`
	Name        string     `json:"name"`
	Email       string     `json:"email"`
	Phone       string     `json:"phone"`
	Status      string     `json:"status"`
	BlockReason string     `json:"block_reason"`
	BlockedAt   *time.Time `json:"blocked_at"`
	LastLoginAt *time.Time `json:"last_login_at"`
	CreatedAt   time.Time  `json:"created_at"`
	OrdersCount int        `json:"orders_count"`
	TotalSpent  float64    `json:"total_spent"`
}

// AdminOrderBrief is a short order summary shown on the customer detail page.
type AdminOrderBrief struct {
	ID            string    `json:"id"`
	OrderNumber   string    `json:"order_number"`
	Status        string    `json:"status"`
	PaymentMethod string    `json:"payment_method"`
	GrandTotal    float64   `json:"grand_total"`
	CreatedAt     time.Time `json:"created_at"`
}

// AdminCustomerDetail is the full customer view for admins.
type AdminCustomerDetail struct {
	AdminCustomer
	Gender         string            `json:"gender"`
	EmailVerified  bool              `json:"email_verified"`
	PhoneVerified  bool              `json:"phone_verified"`
	MarketingOptIn bool              `json:"marketing_opt_in"`
	BlockedByName  string            `json:"blocked_by_name"`
	CancelledCount int               `json:"cancelled_count"`
	TicketsCount   int               `json:"tickets_count"`
	Addresses      []Address         `json:"addresses"`
	RecentOrders   []AdminOrderBrief `json:"recent_orders"`
}

// countedOrders are orders that actually went through (not abandoned/cancelled).
const countedOrders = `o.status NOT IN ('pending_payment','cancelled')`

// ListForAdmin returns customers with order stats, optionally filtered by
// account status ("active" | "blocked") and a free-text search.
func (s *Service) ListForAdmin(ctx context.Context, status, q string, limit, offset int) ([]AdminCustomer, int64, error) {
	status = strings.ToLower(strings.TrimSpace(status))
	q = strings.TrimSpace(q)
	pattern := "%" + q + "%"

	where := `WHERE ($1 = '' OR u.status = $1)
	          AND ($2 = '' OR c.display_name ILIKE $3 OR u.full_name ILIKE $3
	               OR coalesce(u.email,'') ILIKE $3 OR coalesce(u.phone,'') ILIKE $3)`

	var total int64
	if err := s.repo.pool.QueryRow(ctx,
		`SELECT count(*) FROM customers c JOIN users u ON u.id = c.user_id `+where,
		status, q, pattern).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.repo.pool.Query(ctx, `
		SELECT c.id, u.id,
		       coalesce(nullif(c.display_name,''), nullif(u.full_name,''), ''),
		       coalesce(u.email,''), coalesce(u.phone,''), u.status, u.block_reason,
		       u.blocked_at, u.last_login_at, u.created_at,
		       (SELECT count(*) FROM orders o WHERE o.customer_id = c.id AND `+countedOrders+`),
		       (SELECT coalesce(sum(o.grand_total),0) FROM orders o WHERE o.customer_id = c.id AND `+countedOrders+`)
		FROM customers c JOIN users u ON u.id = c.user_id
		`+where+`
		ORDER BY u.created_at DESC
		LIMIT $4 OFFSET $5`, status, q, pattern, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := []AdminCustomer{}
	for rows.Next() {
		var c AdminCustomer
		if err := rows.Scan(&c.ID, &c.UserID, &c.Name, &c.Email, &c.Phone, &c.Status, &c.BlockReason,
			&c.BlockedAt, &c.LastLoginAt, &c.CreatedAt, &c.OrdersCount, &c.TotalSpent); err != nil {
			return nil, 0, err
		}
		out = append(out, c)
	}
	return out, total, rows.Err()
}

// GetForAdmin returns one customer with addresses, stats and recent orders.
func (s *Service) GetForAdmin(ctx context.Context, customerID string) (*AdminCustomerDetail, error) {
	var d AdminCustomerDetail
	err := s.repo.pool.QueryRow(ctx, `
		SELECT c.id, u.id,
		       coalesce(nullif(c.display_name,''), nullif(u.full_name,''), ''),
		       coalesce(u.email,''), coalesce(u.phone,''), u.status, u.block_reason,
		       u.blocked_at, u.last_login_at, u.created_at,
		       coalesce(c.gender,'unspecified'), u.email_verified, u.phone_verified, c.marketing_opt_in,
		       coalesce((SELECT nullif(bu.full_name,'') FROM users bu WHERE bu.id = u.blocked_by),''),
		       (SELECT count(*) FROM orders o WHERE o.customer_id = c.id AND `+countedOrders+`),
		       (SELECT coalesce(sum(o.grand_total),0) FROM orders o WHERE o.customer_id = c.id AND `+countedOrders+`),
		       (SELECT count(*) FROM orders o WHERE o.customer_id = c.id AND o.status = 'cancelled'),
		       (SELECT count(*) FROM support_tickets t WHERE t.customer_id = c.id)
		FROM customers c JOIN users u ON u.id = c.user_id
		WHERE c.id = $1`, customerID).
		Scan(&d.ID, &d.UserID, &d.Name, &d.Email, &d.Phone, &d.Status, &d.BlockReason,
			&d.BlockedAt, &d.LastLoginAt, &d.CreatedAt,
			&d.Gender, &d.EmailVerified, &d.PhoneVerified, &d.MarketingOptIn,
			&d.BlockedByName, &d.OrdersCount, &d.TotalSpent, &d.CancelledCount, &d.TicketsCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("customer_not_found", "Customer not found")
	}
	if err != nil {
		return nil, err
	}

	d.Addresses = []Address{}
	arows, err := s.repo.pool.Query(ctx, `
		SELECT id, label, contact_name, contact_phone, line1, line2, landmark, city, state,
		       pincode, country, address_type, is_default
		FROM customer_addresses WHERE customer_id = $1
		ORDER BY is_default DESC, created_at DESC`, customerID)
	if err != nil {
		return nil, err
	}
	defer arows.Close()
	for arows.Next() {
		var a Address
		if err := arows.Scan(&a.ID, &a.Label, &a.ContactName, &a.ContactPhone, &a.Line1, &a.Line2,
			&a.Landmark, &a.City, &a.State, &a.Pincode, &a.Country, &a.AddressType, &a.IsDefault); err != nil {
			return nil, err
		}
		d.Addresses = append(d.Addresses, a)
	}
	if err := arows.Err(); err != nil {
		return nil, err
	}

	d.RecentOrders = []AdminOrderBrief{}
	orows, err := s.repo.pool.Query(ctx, `
		SELECT o.id, o.order_number, o.status, o.payment_method, o.grand_total, o.created_at
		FROM orders o WHERE o.customer_id = $1
		ORDER BY o.created_at DESC LIMIT 10`, customerID)
	if err != nil {
		return nil, err
	}
	defer orows.Close()
	for orows.Next() {
		var o AdminOrderBrief
		if err := orows.Scan(&o.ID, &o.OrderNumber, &o.Status, &o.PaymentMethod, &o.GrandTotal, &o.CreatedAt); err != nil {
			return nil, err
		}
		d.RecentOrders = append(d.RecentOrders, o)
	}
	return &d, orows.Err()
}

// BlockResult describes the change made, for the audit log.
type BlockResult struct {
	Reason string
}

// Block blocks a customer account with a reason picked from BlockReasons (plus
// an optional note). The customer is signed out everywhere and sees the reason
// the next time they try to log in.
func (s *Service) Block(ctx context.Context, customerID, adminUserID, reasonCode, note string) (*BlockResult, error) {
	r, ok := findBlockReason(strings.TrimSpace(reasonCode))
	if !ok {
		return nil, httpx.BadRequest("validation_failed", "Choose a valid block reason")
	}
	note = strings.TrimSpace(note)
	if r.NoteRequired && len(note) < 3 {
		return nil, httpx.BadRequest("validation_failed", "Please add a short note explaining the reason")
	}
	if len(note) > 300 {
		return nil, httpx.BadRequest("validation_failed", "Note is too long (max 300 characters)")
	}
	reason := r.Label
	if note != "" {
		reason = r.Label + " — " + note
		if r.Code == "other" {
			reason = note
		}
	}

	err := pgx.BeginFunc(ctx, s.repo.pool, func(tx pgx.Tx) error {
		var userID, status string
		err := tx.QueryRow(ctx, `
			SELECT u.id, u.status FROM customers c JOIN users u ON u.id = c.user_id
			WHERE c.id = $1 FOR UPDATE OF u`, customerID).Scan(&userID, &status)
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.NotFound("customer_not_found", "Customer not found")
		}
		if err != nil {
			return err
		}
		if status == "blocked" {
			return httpx.Conflict("already_blocked", "This customer is already blocked")
		}
		if _, err := tx.Exec(ctx, `
			UPDATE users SET status='blocked', block_reason=$2, blocked_at=now(), blocked_by=$3
			WHERE id=$1`, userID, reason, adminUserID); err != nil {
			return err
		}
		// Sign the customer out of every device.
		_, err = tx.Exec(ctx,
			`UPDATE refresh_tokens SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, userID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &BlockResult{Reason: reason}, nil
}

// Unblock restores a blocked customer account.
func (s *Service) Unblock(ctx context.Context, customerID string) error {
	return pgx.BeginFunc(ctx, s.repo.pool, func(tx pgx.Tx) error {
		var userID, status string
		err := tx.QueryRow(ctx, `
			SELECT u.id, u.status FROM customers c JOIN users u ON u.id = c.user_id
			WHERE c.id = $1 FOR UPDATE OF u`, customerID).Scan(&userID, &status)
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.NotFound("customer_not_found", "Customer not found")
		}
		if err != nil {
			return err
		}
		if status != "blocked" {
			return httpx.Conflict("not_blocked", "This customer is not blocked")
		}
		_, err = tx.Exec(ctx, `
			UPDATE users SET status='active', block_reason='', blocked_at=NULL, blocked_by=NULL
			WHERE id=$1`, userID)
		return err
	})
}
