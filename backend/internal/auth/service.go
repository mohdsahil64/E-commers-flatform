package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"

	"github.com/carlitos/backend/internal/httpx"
)

// customerRefreshTTL makes customer sessions effectively non-expiring: the
// refresh token lives ~10 years and is silently rotated on every use, so a
// customer stays logged in until they explicitly log out (or log in elsewhere,
// which revokes prior sessions — single active session).
const customerRefreshTTL = 10 * 365 * 24 * time.Hour

// passwordResetTTL is how long a reset link stays valid.
const passwordResetTTL = 30 * time.Minute

// PasswordResetEmail carries everything the delivery hook needs to send a
// reset email. The hook builds the templated email and dispatches it.
type PasswordResetEmail struct {
	To         string
	Name       string
	Portal     string // seller | admin
	Token      string // raw token to embed in the reset link
	ExpiryMins int
}

// PasswordResetSender delivers a reset email. It runs best-effort off the
// request path; the wiring layer supplies the concrete implementation.
type PasswordResetSender func(ctx context.Context, e PasswordResetEmail)

// Service holds auth business logic.
type Service struct {
	repo        *Repository
	tokens      *Manager
	bcryptCost  int
	otpStore    *OTPStore
	otpSender   OTPSender
	resetSender PasswordResetSender
	verifyEmailSender VerifyEmailSender
}

// sellerEmailKey / sellerPhoneKey scope signup verification challenges so an
// email code can't verify a phone and vice-versa.
func sellerEmailKey(email string) string { return "sellersignup:email:" + normalizeEmail(email) }
func sellerPhoneKey(phone string) string { return "sellersignup:phone:" + normalizePhone(phone) }

// NewService builds an auth Service.
func NewService(repo *Repository, tokens *Manager, bcryptCost int) *Service {
	return &Service{repo: repo, tokens: tokens, bcryptCost: bcryptCost}
}

// WithOTP attaches the OTP store and sender used for customer mobile login.
func (s *Service) WithOTP(store *OTPStore, sender OTPSender) *Service {
	s.otpStore = store
	s.otpSender = sender
	return s
}

// WithPasswordResetSender attaches the email delivery hook for password resets.
func (s *Service) WithPasswordResetSender(sender PasswordResetSender) *Service {
	s.resetSender = sender
	return s
}

// VerifyEmailSender delivers a verification-code email (subject + HTML body).
type VerifyEmailSender func(ctx context.Context, to, subject, htmlBody string) error

// WithEmailVerifySender attaches the email sender used for seller-signup email
// verification codes. Optional; if unset the email-verify endpoint returns 501.
func (s *Service) WithEmailVerifySender(sender VerifyEmailSender) *Service {
	s.verifyEmailSender = sender
	return s
}

// TokenPair is returned on successful auth.
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	User         AuthUser  `json:"user"`
}

// AuthUser is the safe user projection returned to clients.
type AuthUser struct {
	ID          string   `json:"id"`
	AccountType string   `json:"account_type"`
	Email       string   `json:"email"`
	FullName    string   `json:"full_name"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
}

// RegisterCustomerInput is the payload to create a customer account.
type RegisterCustomerInput struct {
	FullName string
	Email    string
	Phone    string
	Password string
}

// RegisterCustomer creates a user (customer), its customer profile, cart and
// wishlist in a single transaction, then issues tokens.
func (s *Service) RegisterCustomer(ctx context.Context, in RegisterCustomerInput, ua, ip string) (*TokenPair, error) {
	email := normalizeEmail(in.Email)
	exists, err := s.repo.EmailExists(ctx, "customer", email)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, httpx.Conflict("email_taken", "An account with this email already exists")
	}

	hash, err := HashPassword(in.Password, s.bcryptCost)
	if err != nil {
		return nil, httpx.Internal("failed to hash password")
	}

	var userID string
	err = pgx.BeginFunc(ctx, s.repo.Pool(), func(tx pgx.Tx) error {
		id, err := s.repo.CreateUser(ctx, tx, UserRecord{
			AccountType:  "customer",
			Email:        email,
			Phone:        in.Phone,
			PasswordHash: hash,
			FullName:     in.FullName,
			Status:       "active",
		})
		if err != nil {
			return err
		}
		userID = id
		var customerID string
		if err := tx.QueryRow(ctx,
			`INSERT INTO customers (user_id, display_name) VALUES ($1,$2) RETURNING id`,
			id, in.FullName).Scan(&customerID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO carts (customer_id) VALUES ($1)`, customerID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO wishlists (customer_id) VALUES ($1)`, customerID); err != nil {
			return err
		}
		return s.repo.AssignRoleByCode(ctx, tx, id, RoleCustomer)
	})
	if err != nil {
		return nil, err
	}

	return s.issueForUser(ctx, userID, "customer", email, ua, ip)
}

// RegisterSellerInput is the payload to create a seller account + store.
type RegisterSellerInput struct {
	OwnerName string
	Email     string
	Phone     string
	Password  string
	StoreName string
}

// RegisterSeller creates a user (seller), a seller store record in 'pending'
// status, and its onboarding record, then issues tokens. The seller cannot
// access seller-desk features until approved (enforced at those endpoints).
func (s *Service) RegisterSeller(ctx context.Context, in RegisterSellerInput, ua, ip string) (*TokenPair, error) {
	email := normalizeEmail(in.Email)
	phone := normalizePhone(in.Phone)
	// Both the email and mobile must have been OTP-verified in the signup flow.
	if s.otpStore != nil {
		if !s.otpStore.IsVerified(sellerEmailKey(email)) {
			return nil, httpx.BadRequest("email_not_verified", "Please verify your email before continuing")
		}
		if !s.otpStore.IsVerified(sellerPhoneKey(phone)) {
			return nil, httpx.BadRequest("phone_not_verified", "Please verify your mobile number before continuing")
		}
	}
	exists, err := s.repo.EmailExists(ctx, "seller", email)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, httpx.Conflict("email_taken", "A seller account with this email already exists")
	}
	hash, err := HashPassword(in.Password, s.bcryptCost)
	if err != nil {
		return nil, httpx.Internal("failed to hash password")
	}

	slug := slugify(in.StoreName)
	var userID string
	err = pgx.BeginFunc(ctx, s.repo.Pool(), func(tx pgx.Tx) error {
		id, err := s.repo.CreateUser(ctx, tx, UserRecord{
			AccountType:  "seller",
			Email:        email,
			Phone:        phone,
			PasswordHash: hash,
			FullName:     in.OwnerName,
			Status:       "active",
		})
		if err != nil {
			return err
		}
		userID = id
		// Email + phone were OTP-verified in the signup flow — record that.
		if _, err := tx.Exec(ctx,
			`UPDATE users SET email_verified = TRUE, phone_verified = TRUE WHERE id = $1`, id); err != nil {
			return err
		}
		// Ensure unique store slug.
		uniqueSlug := slug
		var sellerID string
		if err := tx.QueryRow(ctx, `
			INSERT INTO sellers (user_id, store_name, store_slug, owner_name, contact_email, contact_phone, status)
			VALUES ($1,$2, $3 || '-' || substr(md5(random()::text),1,6), $4,$5,$6,'pending')
			RETURNING id`,
			id, in.StoreName, uniqueSlug, in.OwnerName, email, phone).Scan(&sellerID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO seller_onboarding (seller_id, step) VALUES ($1,'owner')`, sellerID); err != nil {
			return err
		}
		return s.repo.AssignRoleByCode(ctx, tx, id, RoleSeller)
	})
	if err != nil {
		return nil, err
	}
	// Consume the one-time verification markers so they can't be reused.
	if s.otpStore != nil {
		s.otpStore.ClearVerified(sellerEmailKey(email))
		s.otpStore.ClearVerified(sellerPhoneKey(phone))
	}
	return s.issueForUser(ctx, userID, "seller", email, ua, ip)
}

// RequestSellerEmailOTP sends a verification code to a seller-signup email. It
// rejects emails already registered to a seller so the code isn't wasted.
func (s *Service) RequestSellerEmailOTP(ctx context.Context, email string) error {
	if s.otpStore == nil || s.verifyEmailSender == nil {
		return httpx.Internal("Email verification is not configured")
	}
	email = normalizeEmail(email)
	if !emailRe.MatchString(email) {
		return httpx.BadRequest("invalid_email", "Enter a valid email address")
	}
	exists, err := s.repo.EmailExists(ctx, "seller", email)
	if err != nil {
		return err
	}
	if exists {
		return httpx.Conflict("email_taken", "A seller account with this email already exists")
	}
	code := generateOTPCode()
	s.otpStore.Put(sellerEmailKey(email), code)
	subject := "Verify your CARLITOS seller email"
	body := s.sellerVerifyEmailHTML(code)
	if err := s.verifyEmailSender(ctx, email, subject, body); err != nil {
		return httpx.Internal("Could not send verification email")
	}
	return nil
}

// VerifySellerEmailOTP validates the email code and, on success, records a
// short-lived "verified" marker consumed later by RegisterSeller.
func (s *Service) VerifySellerEmailOTP(ctx context.Context, email, code string) error {
	if s.otpStore == nil {
		return httpx.Internal("Email verification is not configured")
	}
	email = normalizeEmail(email)
	if !s.otpStore.Verify(sellerEmailKey(email), strings.TrimSpace(code)) {
		return httpx.Unauthorized("invalid_otp", "Invalid or expired code")
	}
	s.otpStore.MarkVerified(sellerEmailKey(email))
	return nil
}

// RequestSellerPhoneOTP sends a mobile verification code for seller signup.
func (s *Service) RequestSellerPhoneOTP(ctx context.Context, phone string) error {
	if s.otpStore == nil || s.otpSender == nil {
		return httpx.Internal("OTP is not configured")
	}
	p := normalizePhone(phone)
	if !validPhone(p) {
		return httpx.BadRequest("invalid_phone", "Enter a valid mobile number")
	}
	code := generateOTPCode()
	s.otpStore.Put(sellerPhoneKey(p), code)
	_ = s.otpSender.Send(ctx, p, code)
	return nil
}

// VerifySellerPhoneOTP validates the mobile code and records the marker.
func (s *Service) VerifySellerPhoneOTP(ctx context.Context, phone, code string) error {
	if s.otpStore == nil {
		return httpx.Internal("OTP is not configured")
	}
	p := normalizePhone(phone)
	if !validPhone(p) {
		return httpx.BadRequest("invalid_phone", "Enter a valid mobile number")
	}
	if !s.otpStore.Verify(sellerPhoneKey(p), strings.TrimSpace(code)) {
		return httpx.Unauthorized("invalid_otp", "Invalid or expired code")
	}
	s.otpStore.MarkVerified(sellerPhoneKey(p))
	return nil
}

// sellerVerifyEmailHTML renders the seller email-verification message.
func (s *Service) sellerVerifyEmailHTML(code string) string {
	return "<div style=\"font-family:sans-serif\"><h2>Verify your email</h2>" +
		"<p>Use this code to verify your CARLITOS seller email:</p>" +
		"<p style=\"font-size:32px;font-weight:700;letter-spacing:8px\">" + code + "</p>" +
		"<p>The code is valid for a few minutes. If you didn't request this, ignore this email.</p></div>"
}

// Login authenticates a user within a portal (account_type) and issues tokens.
func (s *Service) Login(ctx context.Context, accountType, email, password, ua, ip string) (*TokenPair, error) {
	email = normalizeEmail(email)
	u, err := s.repo.FindUserByEmail(ctx, accountType, email)
	if errors.Is(err, ErrNotFound) {
		return nil, httpx.Unauthorized("invalid_credentials", "Invalid email or password")
	}
	if err != nil {
		return nil, err
	}
	if !CheckPassword(u.PasswordHash, password) {
		return nil, httpx.Unauthorized("invalid_credentials", "Invalid email or password")
	}
	switch u.Status {
	case "suspended":
		return nil, httpx.Forbidden("account_suspended", "This account is suspended")
	case "blocked":
		return nil, httpx.Forbidden("account_blocked", "This account is blocked")
	case "inactive":
		return nil, httpx.Forbidden("account_inactive", "This account is inactive")
	}
	return s.issueForUser(ctx, u.ID, u.AccountType, u.Email, ua, ip)
}

// RequestOTP issues a one-time code for a customer phone number and sends it
// via the configured OTP sender. It intentionally does not reveal whether the
// phone belongs to an existing account (no user enumeration). In development
// the sender logs the code and 123456 is always accepted at verification.
func (s *Service) RequestOTP(ctx context.Context, phone string) error {
	if s.otpStore == nil || s.otpSender == nil {
		return httpx.Internal("OTP login is not configured")
	}
	p := normalizePhone(phone)
	if !validPhone(p) {
		return httpx.BadRequest("invalid_phone", "Enter a valid mobile number")
	}
	code := generateOTPCode()
	s.otpStore.Put(p, code)
	// Best-effort send; failures are logged by the sender. We do not surface
	// delivery errors to avoid leaking provider details.
	_ = s.otpSender.Send(ctx, p, code)
	return nil
}

// VerifyOTP validates a phone + code, finding or creating the customer account,
// then issues tokens. This is the single entry point for customer login and
// registration via mobile number.
func (s *Service) VerifyOTP(ctx context.Context, phone, code, ua, ip string) (*TokenPair, error) {
	if s.otpStore == nil {
		return nil, httpx.Internal("OTP login is not configured")
	}
	p := normalizePhone(phone)
	if !validPhone(p) {
		return nil, httpx.BadRequest("invalid_phone", "Enter a valid mobile number")
	}
	if !s.otpStore.Verify(p, strings.TrimSpace(code)) {
		return nil, httpx.Unauthorized("invalid_otp", "Invalid or expired code")
	}

	// Find existing customer by phone, or create one on first login.
	u, err := s.repo.FindUserByPhone(ctx, "customer", p)
	if err == nil {
		switch u.Status {
		case "suspended":
			return nil, httpx.Forbidden("account_suspended", "This account is suspended")
		case "blocked":
			return nil, httpx.Forbidden("account_blocked", "This account is blocked")
		case "inactive":
			return nil, httpx.Forbidden("account_inactive", "This account is inactive")
		}
		_ = s.repo.MarkPhoneVerified(ctx, u.ID)
		// Single active session for customers: revoke any existing sessions so
		// the account can only be open on one device/phone at a time. Logging in
		// on a new phone invalidates the previous one.
		_ = s.repo.RevokeAllForUser(ctx, u.ID)
		return s.issueForUser(ctx, u.ID, "customer", u.Email, ua, ip)
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	// First-time login: create the customer account keyed on the phone number.
	// Email is left NULL — the mobile number is the primary identity. The
	// customer can add an (OTP-verified) email later from their profile. A random
	// password hash is stored so password login is effectively disabled.
	randomHash, herr := HashPassword(uuidLike(), s.bcryptCost)
	if herr != nil {
		return nil, httpx.Internal("failed to create account")
	}
	var userID string
	err = pgx.BeginFunc(ctx, s.repo.Pool(), func(tx pgx.Tx) error {
		id, err := s.repo.CreateUser(ctx, tx, UserRecord{
			AccountType:  "customer",
			Email:        "", // no email at signup — phone is the identity
			Phone:        p,
			PasswordHash: randomHash,
			FullName:     "",
			Status:       "active",
		})
		if err != nil {
			return err
		}
		userID = id
		var customerID string
		if err := tx.QueryRow(ctx,
			`INSERT INTO customers (user_id, display_name) VALUES ($1,$2) RETURNING id`,
			id, "").Scan(&customerID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO carts (customer_id) VALUES ($1)`, customerID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO wishlists (customer_id) VALUES ($1)`, customerID); err != nil {
			return err
		}
		return s.repo.AssignRoleByCode(ctx, tx, id, RoleCustomer)
	})
	if err != nil {
		return nil, err
	}
	_ = s.repo.MarkPhoneVerified(ctx, userID)
	return s.issueForUser(ctx, userID, "customer", "", ua, ip)
}

// Refresh rotates a refresh token: validates the old one, revokes it, and
// issues a fresh pair.
func (s *Service) Refresh(ctx context.Context, rawRefresh, ua, ip string) (*TokenPair, error) {
	hash := HashToken(rawRefresh)
	userID, err := s.repo.FindValidRefreshToken(ctx, hash)
	if errors.Is(err, ErrNotFound) {
		return nil, httpx.Unauthorized("invalid_refresh", "Invalid or expired refresh token")
	}
	if err != nil {
		return nil, err
	}
	_ = s.repo.RevokeRefreshToken(ctx, hash)
	u, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, httpx.Unauthorized("invalid_refresh", "User no longer exists")
	}
	return s.issueForUser(ctx, u.ID, u.AccountType, u.Email, ua, ip)
}

// Logout revokes a refresh token.
func (s *Service) Logout(ctx context.Context, rawRefresh string) error {
	return s.repo.RevokeRefreshToken(ctx, HashToken(rawRefresh))
}

// RequestPasswordReset starts the forgot-password flow for an email/password
// portal (seller or admin). To avoid account enumeration it always succeeds
// from the caller's perspective: if the email has no account, nothing is sent
// and no error is returned. When an account exists, a single-use token is
// stored (hashed) and the reset email is dispatched via the configured sender.
func (s *Service) RequestPasswordReset(ctx context.Context, accountType, email, ua, ip string) error {
	if accountType != "seller" && accountType != "admin" {
		return httpx.BadRequest("unsupported_portal", "Password reset is not available for this account type")
	}
	email = normalizeEmail(email)
	if !emailRe.MatchString(email) {
		return httpx.BadRequest("invalid_email", "Enter a valid email address")
	}

	u, err := s.repo.FindUserByEmail(ctx, accountType, email)
	if errors.Is(err, ErrNotFound) {
		// Silently succeed — do not reveal whether the email is registered.
		return nil
	}
	if err != nil {
		return err
	}
	// Blocked/suspended accounts cannot reset; stay silent to avoid enumeration.
	if u.Status == "blocked" || u.Status == "suspended" {
		return nil
	}

	rawToken := generateResetToken()
	exp := time.Now().Add(passwordResetTTL)
	if err := s.repo.CreatePasswordResetToken(ctx, u.ID, HashToken(rawToken), exp, ua, ip); err != nil {
		return err
	}

	if s.resetSender != nil {
		s.resetSender(ctx, PasswordResetEmail{
			To:         u.Email,
			Name:       u.FullName,
			Portal:     accountType,
			Token:      rawToken,
			ExpiryMins: int(passwordResetTTL / time.Minute),
		})
	}
	return nil
}

// ResetPassword completes the flow: it validates the single-use token, hashes
// and stores the new password, consumes the token, and revokes existing
// sessions. Returns a generic error if the token is invalid or expired.
func (s *Service) ResetPassword(ctx context.Context, rawToken, newPassword string) error {
	if len(newPassword) < 8 {
		return httpx.BadRequest("weak_password", "Password must be at least 8 characters")
	}
	tokenHash := HashToken(strings.TrimSpace(rawToken))
	userID, err := s.repo.FindValidPasswordResetToken(ctx, tokenHash)
	if errors.Is(err, ErrNotFound) {
		return httpx.BadRequest("invalid_reset_token", "This reset link is invalid or has expired")
	}
	if err != nil {
		return err
	}
	hash, err := HashPassword(newPassword, s.bcryptCost)
	if err != nil {
		return httpx.Internal("failed to hash password")
	}
	if err := s.repo.ConsumePasswordReset(ctx, tokenHash, userID, hash); err != nil {
		if errors.Is(err, ErrNotFound) {
			return httpx.BadRequest("invalid_reset_token", "This reset link is invalid or has expired")
		}
		return err
	}
	return nil
}

// generateResetToken returns a URL-safe random token used in reset links.
func generateResetToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return hex.EncodeToString([]byte(fmt.Sprintf("reset-%d", timeNowUnixNano())))
	}
	return hex.EncodeToString(b)
}

// Me returns the current user's profile projection.
func (s *Service) Me(ctx context.Context, userID string) (*AuthUser, error) {
	u, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, httpx.NotFound("user_not_found", "User not found")
	}
	roles, perms, err := s.repo.LoadRolesAndPermissions(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &AuthUser{
		ID: u.ID, AccountType: u.AccountType, Email: u.Email, FullName: u.FullName,
		Roles: roles, Permissions: perms,
	}, nil
}

// issueForUser loads roles/permissions, mints an access token + refresh token,
// stores the refresh hash, and returns the pair.
func (s *Service) issueForUser(ctx context.Context, userID, accountType, email, ua, ip string) (*TokenPair, error) {
	roles, perms, err := s.repo.LoadRolesAndPermissions(ctx, userID)
	if err != nil {
		return nil, err
	}
	access, exp, err := s.tokens.IssueAccess(userID, accountType, email, roles, perms)
	if err != nil {
		return nil, httpx.Internal("failed to issue token")
	}
	// Customers get an effectively non-expiring session (long refresh TTL +
	// silent rotation on each use), like consumer shopping apps. Seller/admin
	// keep the standard TTL from config.
	var rawRefresh, refreshHash string
	var refreshExp time.Time
	if accountType == "customer" {
		rawRefresh, refreshHash, refreshExp, err = s.tokens.IssueRefreshWithTTL(customerRefreshTTL)
	} else {
		rawRefresh, refreshHash, refreshExp, err = s.tokens.IssueRefresh()
	}
	if err != nil {
		return nil, httpx.Internal("failed to issue refresh token")
	}
	if err := s.repo.StoreRefreshToken(ctx, userID, refreshHash, ua, ip, refreshExp); err != nil {
		return nil, err
	}
	_ = s.repo.TouchLastLogin(ctx, userID)

	return &TokenPair{
		AccessToken:  access,
		RefreshToken: rawRefresh,
		ExpiresAt:    exp,
		User: AuthUser{
			ID: userID, AccountType: accountType, Email: email,
			Roles: roles, Permissions: perms,
		},
	}, nil
}

func normalizeEmail(e string) string { return strings.ToLower(strings.TrimSpace(e)) }

// normalizePhone strips spaces, dashes and parentheses, keeping a leading '+'
// and digits. This gives a stable identifier regardless of input formatting.
func normalizePhone(p string) string {
	p = strings.TrimSpace(p)
	var b strings.Builder
	for i, r := range p {
		if r == '+' && i == 0 {
			b.WriteRune(r)
			continue
		}
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// validPhone accepts 10–15 digits, with an optional leading '+'.
func validPhone(p string) bool {
	digits := p
	if strings.HasPrefix(digits, "+") {
		digits = digits[1:]
	}
	if len(digits) < 10 || len(digits) > 15 {
		return false
	}
	for _, r := range digits {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// generateOTPCode returns a 6-digit numeric code. In development the sender
// logs it and the store also accepts the fixed DevOTPCode, so testers can use
// 123456 without reading logs.
func generateOTPCode() string { return GenerateOTPCode() }

// GenerateOTPCode returns a 6-digit numeric verification code (exported so other
// modules — e.g. customer email verification — reuse the same generator).
func GenerateOTPCode() string {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return DevOTPCode
	}
	return fmt.Sprintf("%06d", n.Int64())
}

// uuidLike returns random bytes rendered as a hex string, used as a throwaway
// password for OTP-only accounts (password login is effectively disabled).
func uuidLike() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("otp-%d", timeNowUnixNano())
	}
	return hex.EncodeToString(b)
}

func timeNowUnixNano() int64 { return time.Now().UnixNano() }

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash {
				b.WriteRune('-')
				prevDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "store"
	}
	return out
}

// clientMeta extracts user agent + IP from a Fiber ctx (shared helper).
func clientMeta(c *fiber.Ctx) (ua, ip string) {
	return c.Get("User-Agent"), c.IP()
}
