// Package customers manages the customer profile, addresses and notification
// preferences.
package customers

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/carlitos/backend/internal/emailtpl"
	"github.com/carlitos/backend/internal/httpx"
)

// Profile is the customer profile view model.
type Profile struct {
	ID           string `json:"id"`
	DisplayName  string `json:"display_name"`
	Email        string `json:"email"`
	Phone        string `json:"phone"`
	Gender       string `json:"gender"`
	NotifyEmail  bool   `json:"notify_email"`
	NotifySMS    bool   `json:"notify_sms"`
	NotifyPush   bool   `json:"notify_push"`
	MarketingOptIn bool `json:"marketing_opt_in"`
}

// Address is a customer address.
type Address struct {
	ID           string `json:"id"`
	Label        string `json:"label"`
	ContactName  string `json:"contact_name"`
	ContactPhone string `json:"contact_phone"`
	Line1        string `json:"line1"`
	Line2        string `json:"line2"`
	Landmark     string `json:"landmark"`
	City         string `json:"city"`
	State        string `json:"state"`
	Pincode      string `json:"pincode"`
	Country      string `json:"country"`
	AddressType  string `json:"address_type"`
	IsDefault    bool   `json:"is_default"`
}

// Repository provides customer persistence.
type Repository struct{ pool *pgxpool.Pool }

// NewRepository builds a customer Repository.
func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// EmailOTP abstracts the one-time-code store used for email verification. It is
// satisfied by auth.OTPStore (Put/Verify), so customers reuse the same store.
type EmailOTP interface {
	Put(key, code string)
	Verify(key, code string) bool
}

// EmailSender sends a verification email (subject + HTML body) to an address.
type EmailSender func(ctx context.Context, to, subject, htmlBody string) error

// CodeGen generates a numeric verification code.
type CodeGen func() string

// Service holds customer business logic.
type Service struct {
	repo    *Repository
	otp     EmailOTP
	sendEml EmailSender
	genCode CodeGen
}

// NewService builds a customer Service.
func NewService(repo *Repository) *Service { return &Service{repo: repo} }

// WithEmailVerification wires the OTP store, email sender and code generator used
// for the add/change-email flow. Optional; if unset those endpoints return 501.
func (s *Service) WithEmailVerification(store EmailOTP, sender EmailSender, gen CodeGen) *Service {
	s.otp = store
	s.sendEml = sender
	s.genCode = gen
	return s
}

func (r *Repository) customerID(ctx context.Context, userID string) (string, error) {
	var id string
	err := r.pool.QueryRow(ctx, `SELECT id FROM customers WHERE user_id = $1`, userID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", httpx.NotFound("customer_not_found", "Customer not found")
	}
	return id, err
}

// GetProfile returns the customer's profile.
func (s *Service) GetProfile(ctx context.Context, userID string) (*Profile, error) {
	var p Profile
	err := s.repo.pool.QueryRow(ctx, `
		SELECT c.id, c.display_name, coalesce(u.email,''), coalesce(u.phone,''), c.gender,
		       c.notify_email, c.notify_sms, c.notify_push, c.marketing_opt_in
		FROM customers c JOIN users u ON u.id = c.user_id
		WHERE c.user_id = $1`, userID).
		Scan(&p.ID, &p.DisplayName, &p.Email, &p.Phone, &p.Gender,
			&p.NotifyEmail, &p.NotifySMS, &p.NotifyPush, &p.MarketingOptIn)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("customer_not_found", "Customer not found")
	}
	return &p, err
}

// UpdateProfile updates display name / gender / notification preferences.
func (s *Service) UpdateProfile(ctx context.Context, userID string, p Profile) error {
	cid, err := s.repo.customerID(ctx, userID)
	if err != nil {
		return err
	}
	_, err = s.repo.pool.Exec(ctx, `
		UPDATE customers SET display_name=$2, gender=COALESCE(NULLIF($3,''),gender),
			notify_email=$4, notify_sms=$5, notify_push=$6, marketing_opt_in=$7
		WHERE id=$1`,
		cid, p.DisplayName, p.Gender, p.NotifyEmail, p.NotifySMS, p.NotifyPush, p.MarketingOptIn)
	return err
}

// RequestEmailChange validates the email, ensures it's not used by another
// customer, generates a verification code (keyed by user+email) and emails it.
// The mobile number is never touched here — only email is add/changeable.
func (s *Service) RequestEmailChange(ctx context.Context, userID, email string) error {
	if s.otp == nil || s.sendEml == nil {
		return httpx.Internal("Email verification is not configured")
	}
	email = normalizeEmail(email)
	if !validEmail(email) {
		return httpx.BadRequest("invalid_email", "Enter a valid email address")
	}
	// Reject if another customer already uses this email.
	var taken bool
	if err := s.repo.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM users WHERE account_type='customer' AND email=$1 AND id<>$2)`,
		email, userID).Scan(&taken); err != nil {
		return err
	}
	if taken {
		return httpx.Conflict("email_taken", "This email is already linked to another account")
	}
	code := s.genCode()
	s.otp.Put(emailKey(userID, email), code)
	subject := "Verify your CARLITOS email"
	body := emailOTPHTML(code)
	if err := s.sendEml(ctx, email, subject, body); err != nil {
		return httpx.Internal("Could not send verification email")
	}
	return nil
}

// ConfirmEmailChange verifies the code for (userID,email) and, on success, sets
// the user's email and marks it verified.
func (s *Service) ConfirmEmailChange(ctx context.Context, userID, email, code string) error {
	if s.otp == nil {
		return httpx.Internal("Email verification is not configured")
	}
	email = normalizeEmail(email)
	if !validEmail(email) {
		return httpx.BadRequest("invalid_email", "Enter a valid email address")
	}
	if !s.otp.Verify(emailKey(userID, email), code) {
		return httpx.Unauthorized("invalid_otp", "Invalid or expired code")
	}
	// Guard against a race where the email got taken between request and confirm.
	ct, err := s.repo.pool.Exec(ctx, `
		UPDATE users SET email=$2, email_verified=TRUE
		WHERE id=$1
		  AND NOT EXISTS (SELECT 1 FROM users WHERE account_type='customer' AND email=$2 AND id<>$1)`,
		userID, email)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return httpx.Conflict("email_taken", "This email is already linked to another account")
	}
	return nil
}

// ListAddresses returns a customer's addresses.
func (s *Service) ListAddresses(ctx context.Context, userID string) ([]Address, error) {
	cid, err := s.repo.customerID(ctx, userID)
	if err != nil {
		return nil, err
	}
	rows, err := s.repo.pool.Query(ctx, `
		SELECT id, label, contact_name, contact_phone, line1, line2, landmark,
		       city, state, pincode, country, address_type, is_default
		FROM customer_addresses WHERE customer_id = $1 ORDER BY is_default DESC, created_at DESC`, cid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Address
	for rows.Next() {
		var a Address
		if err := rows.Scan(&a.ID, &a.Label, &a.ContactName, &a.ContactPhone, &a.Line1,
			&a.Line2, &a.Landmark, &a.City, &a.State, &a.Pincode, &a.Country,
			&a.AddressType, &a.IsDefault); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// AddAddress creates an address; if it's the first or marked default, it becomes
// the default (and unsets any previous default) atomically.
func (s *Service) AddAddress(ctx context.Context, userID string, a Address) (string, error) {
	cid, err := s.repo.customerID(ctx, userID)
	if err != nil {
		return "", err
	}
	if a.ContactName == "" || a.Line1 == "" || a.City == "" || a.Pincode == "" {
		return "", httpx.BadRequest("validation_failed", "Name, address line, city and pincode are required")
	}
	var id string
	err = pgx.BeginFunc(ctx, s.repo.pool, func(tx pgx.Tx) error {
		var count int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM customer_addresses WHERE customer_id=$1`, cid).Scan(&count); err != nil {
			return err
		}
		makeDefault := a.IsDefault || count == 0
		if makeDefault {
			if _, err := tx.Exec(ctx, `UPDATE customer_addresses SET is_default=FALSE WHERE customer_id=$1`, cid); err != nil {
				return err
			}
		}
		if err := tx.QueryRow(ctx, `
			INSERT INTO customer_addresses (customer_id, label, contact_name, contact_phone,
				line1, line2, landmark, city, state, pincode, country, address_type, is_default)
			VALUES ($1,COALESCE(NULLIF($2,''),'Home'),$3,$4,$5,$6,$7,$8,$9,$10,COALESCE(NULLIF($11,''),'India'),COALESCE(NULLIF($12,''),'home'),$13)
			RETURNING id`,
			cid, a.Label, a.ContactName, a.ContactPhone, a.Line1, a.Line2, a.Landmark,
			a.City, a.State, a.Pincode, a.Country, a.AddressType, makeDefault).Scan(&id); err != nil {
			return err
		}
		if makeDefault {
			_, err := tx.Exec(ctx, `UPDATE customers SET default_address_id=$2 WHERE id=$1`, cid, id)
			return err
		}
		return nil
	})
	return id, err
}

// SetDefaultAddress marks an address as the customer's default.
func (s *Service) SetDefaultAddress(ctx context.Context, userID, addressID string) error {
	cid, err := s.repo.customerID(ctx, userID)
	if err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, s.repo.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE customer_addresses SET is_default=FALSE WHERE customer_id=$1`, cid); err != nil {
			return err
		}
		ct, err := tx.Exec(ctx, `UPDATE customer_addresses SET is_default=TRUE WHERE id=$1 AND customer_id=$2`, addressID, cid)
		if err != nil {
			return err
		}
		if ct.RowsAffected() == 0 {
			return httpx.NotFound("address_not_found", "Address not found")
		}
		_, err = tx.Exec(ctx, `UPDATE customers SET default_address_id=$2 WHERE id=$1`, cid, addressID)
		return err
	})
}

// DeleteAddress removes an address.
func (s *Service) DeleteAddress(ctx context.Context, userID, addressID string) error {
	cid, err := s.repo.customerID(ctx, userID)
	if err != nil {
		return err
	}
	_, err = s.repo.pool.Exec(ctx, `DELETE FROM customer_addresses WHERE id=$1 AND customer_id=$2`, addressID, cid)
	return err
}

// ── email verification helpers ───────────────────────────────────────────────

func normalizeEmail(e string) string { return strings.ToLower(strings.TrimSpace(e)) }

// validEmail is a pragmatic check (single @, non-empty local/domain, a dot in
// the domain). Full RFC validation is unnecessary for a verification flow that
// also requires the recipient to receive the code.
func validEmail(e string) bool {
	at := strings.IndexByte(e, '@')
	if at <= 0 || at != strings.LastIndexByte(e, '@') || at == len(e)-1 {
		return false
	}
	domain := e[at+1:]
	return strings.Contains(domain, ".") && !strings.HasPrefix(domain, ".") && !strings.HasSuffix(domain, ".")
}

// emailKey scopes an OTP challenge to a specific user + target email so a code
// issued for one email can't verify a different one.
func emailKey(userID, email string) string { return "emailchange:" + userID + ":" + email }

// emailOTPHTML renders the verification-code email using the shared template.
func emailOTPHTML(code string) string {
	return emailtpl.SellerOTP(emailtpl.SellerOTPData{
		OwnerName:  "there",
		OTP:        code,
		ExpiryMins: 5,
		Purpose:    "verify your email address",
	})
}
