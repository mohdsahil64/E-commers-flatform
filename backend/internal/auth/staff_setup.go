package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/carlitos/backend/internal/emailtpl"
	"github.com/carlitos/backend/internal/httpx"
)

// First-login setup for invited admin employees.
//
// An invited employee only holds a one-time TEMPORARY password. Knowing it is
// not enough to get a session. The flow is:
//
//  1. POST /auth/admin/login with the temporary password. If the password is
//     right and the account is still awaiting setup, NO tokens are issued.
//     Instead a short-lived setup token is returned and a 6-digit code is
//     emailed to the employee.
//  2. POST /auth/admin/setup/verify with {setup_token, code, new_password}.
//     The code is checked (hashed, expiring, attempt-limited), the new password
//     is validated and saved, the account becomes active and a session starts.
//
// The setup token and the code are stored hashed; the code is never logged
// (except on a development box where e-mail is not really delivered).
const (
	setupTokenTTL    = 30 * time.Minute
	setupCodeTTL     = 10 * time.Minute
	maxSetupAttempts = 5
	maxSetupResends  = 5
	setupResendGap   = 60 * time.Second
)

// StaffEventHook lets the wiring layer audit staff lifecycle events without
// the auth package depending on the audit package.
type StaffEventHook func(ctx context.Context, userID, event string)

// WithStaffHook attaches the staff lifecycle hook (optional).
func (s *Service) WithStaffHook(h StaffEventHook) *Service {
	s.staffHook = h
	return s
}

// WithDevCredentialLogging makes the service log first-login codes. Pass a
// logger ONLY when e-mail is not really delivered (the dev "log" provider), so
// a developer can still finish the flow locally. Never pass one in production.
func (s *Service) WithDevCredentialLogging(l *slog.Logger) *Service {
	s.devLogger = l
	return s
}

// SetupRequired is returned instead of tokens when an invited employee signs in
// with their temporary password.
type SetupRequired struct {
	SetupRequired bool   `json:"setup_required"`
	SetupToken    string `json:"setup_token"`
	EmailMasked   string `json:"email_masked"`
	Name          string `json:"name"`
	CodeExpiresIn int    `json:"code_expires_in"`  // seconds
	SetupExpires  int    `json:"setup_expires_in"` // seconds
}

// AdminLoginResult is either a finished session or a pending first-login setup.
type AdminLoginResult struct {
	Tokens *TokenPair
	Setup  *SetupRequired
}

// ── login throttling (admin portal) ────────────────────────────────────────
//
// On top of the per-IP request limiter, wrong passwords are counted per
// (email, IP) pair and per email. Too many failures lock further attempts for a
// while, which blunts password guessing against an admin account.

const (
	throttleWindow   = 15 * time.Minute
	throttleLockout  = 15 * time.Minute
	maxPairFailures  = 5
	maxEmailFailures = 20
)

type throttleEntry struct {
	fails       int
	windowStart time.Time
	lockedUntil time.Time
}

type loginThrottle struct {
	mu sync.Mutex
	m  map[string]*throttleEntry
}

func newLoginThrottle() *loginThrottle { return &loginThrottle{m: map[string]*throttleEntry{}} }

// lockedFor returns how long the caller must wait, or 0 when not locked.
func (t *loginThrottle) lockedFor(keys ...string) time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	var wait time.Duration
	now := time.Now()
	for _, k := range keys {
		if e, ok := t.m[k]; ok && now.Before(e.lockedUntil) {
			if d := e.lockedUntil.Sub(now); d > wait {
				wait = d
			}
		}
	}
	return wait
}

func (t *loginThrottle) fail(key string, limit int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	if len(t.m) > 5000 { // opportunistic sweep so the map cannot grow forever
		for k, e := range t.m {
			if now.After(e.lockedUntil) && now.Sub(e.windowStart) > throttleWindow {
				delete(t.m, k)
			}
		}
	}
	e, ok := t.m[key]
	if !ok || now.Sub(e.windowStart) > throttleWindow {
		e = &throttleEntry{windowStart: now}
		t.m[key] = e
	}
	e.fails++
	if e.fails >= limit {
		e.lockedUntil = now.Add(throttleLockout)
		e.fails = 0
		e.windowStart = now
	}
}

func (t *loginThrottle) reset(keys ...string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, k := range keys {
		delete(t.m, k)
	}
}

var (
	dummyHashOnce sync.Once
	dummyHash     string
)

// burnPasswordCheck spends roughly the same time as a real password check so
// that an unknown email is not distinguishable from a wrong password by timing.
func (s *Service) burnPasswordCheck(password string) {
	dummyHashOnce.Do(func() { dummyHash, _ = HashPassword("not-a-real-password-"+uuidLike(), s.bcryptCost) })
	_ = CheckPassword(dummyHash, password)
}

// LoginAdmin authenticates an admin-portal user. It returns a session for a
// normal account, or a SetupRequired challenge for an invited employee who is
// still on the temporary password.
func (s *Service) LoginAdmin(ctx context.Context, email, password, ua, ip string) (*AdminLoginResult, error) {
	email = normalizeEmail(email)
	pairKey := "p:" + email + "|" + ip
	emailKey := "e:" + email
	if wait := s.adminThrott.lockedFor(pairKey, emailKey); wait > 0 {
		mins := int(wait/time.Minute) + 1
		return nil, httpx.NewError(fiber.StatusTooManyRequests, "too_many_attempts",
			fmt.Sprintf("Too many failed sign-in attempts. Try again in %d minute(s).", mins))
	}

	u, err := s.repo.FindUserByEmail(ctx, "admin", email)
	if errors.Is(err, ErrNotFound) {
		s.burnPasswordCheck(password)
		s.adminThrott.fail(pairKey, maxPairFailures)
		s.adminThrott.fail(emailKey, maxEmailFailures)
		return nil, httpx.Unauthorized("invalid_credentials", "Invalid email or password")
	}
	if err != nil {
		return nil, err
	}
	if !CheckPassword(u.PasswordHash, password) {
		s.adminThrott.fail(pairKey, maxPairFailures)
		s.adminThrott.fail(emailKey, maxEmailFailures)
		return nil, httpx.Unauthorized("invalid_credentials", "Invalid email or password")
	}
	switch u.Status {
	case "suspended":
		return nil, httpx.Forbidden("account_suspended", "This account is suspended")
	case "blocked":
		return nil, httpx.Forbidden("account_blocked", "This account is blocked")
	case "inactive":
		return nil, httpx.Forbidden("account_inactive", "This account has been disabled. Contact your administrator.")
	}
	s.adminThrott.reset(pairKey, emailKey)

	gate, err := s.repo.GetStaffGate(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	if gate != nil && gate.MustChangePassword {
		setup, err := s.startStaffSetup(ctx, u, gate, ip)
		if err != nil {
			return nil, err
		}
		return &AdminLoginResult{Setup: setup}, nil
	}
	pair, err := s.issueForUser(ctx, u.ID, u.AccountType, u.Email, ua, ip)
	if err != nil {
		return nil, err
	}
	return &AdminLoginResult{Tokens: pair}, nil
}

func (s *Service) startStaffSetup(ctx context.Context, u *UserRecord, gate *StaffGate, ip string) (*SetupRequired, error) {
	if gate.InviteExpiresAt != nil && time.Now().After(*gate.InviteExpiresAt) {
		return nil, httpx.Forbidden("invitation_expired",
			"Your invitation has expired. Ask your administrator to send a new one.")
	}
	rawToken, err := randomHex(32)
	if err != nil {
		return nil, httpx.Internal("could not start account setup")
	}
	code, err := secureCode()
	if err != nil {
		return nil, httpx.Internal("could not start account setup")
	}
	tokenHash := HashToken(rawToken)
	now := time.Now()
	if err := s.repo.CreateSetupChallenge(ctx, u.ID, tokenHash, hashSetupCode(tokenHash, code),
		now.Add(setupTokenTTL), now.Add(setupCodeTTL), ip); err != nil {
		return nil, err
	}
	if err := s.sendSetupCode(ctx, u.Email, u.FullName, code); err != nil {
		return nil, err
	}
	return &SetupRequired{
		SetupRequired: true,
		SetupToken:    rawToken,
		EmailMasked:   maskEmail(u.Email),
		Name:          u.FullName,
		CodeExpiresIn: int(setupCodeTTL / time.Second),
		SetupExpires:  int(setupTokenTTL / time.Second),
	}, nil
}

func (s *Service) sendSetupCode(ctx context.Context, to, name, code string) error {
	if s.devLogger != nil {
		s.devLogger.Warn("DEV ONLY: e-mail is not delivered — first-login verification code",
			"to", to, "code", code)
	}
	if s.verifyEmailSender == nil {
		return httpx.Internal("Email delivery is not configured")
	}
	html := emailtpl.StaffSetupCode(emailtpl.StaffSetupCodeData{
		Name: name, Code: code, ExpiryMins: int(setupCodeTTL / time.Minute),
	})
	if err := s.verifyEmailSender(ctx, to, "Your CARLITOS verification code", html); err != nil {
		return httpx.Internal("We couldn't send the verification email. Please try again.")
	}
	return nil
}

// CompleteStaffSetup verifies the emailed code, saves the employee's new
// password, activates the account and starts a session.
func (s *Service) CompleteStaffSetup(ctx context.Context, setupToken, code, newPassword, ua, ip string) (*TokenPair, error) {
	tokenHash := HashToken(strings.TrimSpace(setupToken))
	ch, err := s.repo.FindSetupChallenge(ctx, tokenHash)
	if errors.Is(err, ErrNotFound) {
		return nil, httpx.Unauthorized("setup_expired", "Your setup session has expired. Please sign in again.")
	}
	if err != nil {
		return nil, err
	}
	if ch.Attempts >= maxSetupAttempts {
		_ = s.repo.InvalidateSetupChallenge(ctx, ch.ID)
		return nil, httpx.Unauthorized("too_many_attempts", "Too many incorrect codes. Please sign in again.")
	}

	// Password policy first: it involves no secret, so a typo here must not burn
	// one of the code attempts.
	if msg := StaffPasswordError(newPassword, ch.Email); msg != "" {
		return nil, httpx.BadRequest("weak_password", msg)
	}
	if CheckPassword(ch.PasswordHash, newPassword) {
		return nil, httpx.BadRequest("same_password", "Choose a different password from the temporary one")
	}

	if time.Now().After(ch.CodeExpiresAt) {
		return nil, httpx.Unauthorized("code_expired", "This code has expired. Request a new one.")
	}
	want := hashSetupCode(tokenHash, strings.TrimSpace(code))
	if subtle.ConstantTimeCompare([]byte(want), []byte(ch.CodeHash)) != 1 {
		n, _ := s.repo.BumpSetupAttempts(ctx, ch.ID)
		if n >= maxSetupAttempts {
			_ = s.repo.InvalidateSetupChallenge(ctx, ch.ID)
			return nil, httpx.Unauthorized("too_many_attempts", "Too many incorrect codes. Please sign in again.")
		}
		return nil, httpx.Unauthorized("invalid_code",
			fmt.Sprintf("That code is incorrect. %d attempt(s) left.", maxSetupAttempts-n))
	}

	hash, err := HashPassword(newPassword, s.bcryptCost)
	if err != nil {
		return nil, httpx.Internal("failed to hash password")
	}
	if err := s.repo.CompleteStaffSetup(ctx, ch.ID, ch.UserID, hash); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, httpx.Unauthorized("setup_expired", "Your setup session has expired. Please sign in again.")
		}
		return nil, err
	}
	if s.staffHook != nil {
		s.staffHook(ctx, ch.UserID, "staff.accepted")
	}
	return s.issueForUser(ctx, ch.UserID, ch.AccountType, ch.Email, ua, ip)
}

// ResendStaffSetupCode emails a fresh code for an in-progress setup, with a
// cool-down and an overall cap so it cannot be used to spam an inbox.
func (s *Service) ResendStaffSetupCode(ctx context.Context, setupToken string) error {
	tokenHash := HashToken(strings.TrimSpace(setupToken))
	ch, err := s.repo.FindSetupChallenge(ctx, tokenHash)
	if errors.Is(err, ErrNotFound) {
		return httpx.Unauthorized("setup_expired", "Your setup session has expired. Please sign in again.")
	}
	if err != nil {
		return err
	}
	if ch.ResendCount >= maxSetupResends {
		return httpx.NewError(fiber.StatusTooManyRequests, "too_many_resends",
			"Too many codes requested. Please sign in again.")
	}
	if wait := time.Until(ch.LastSentAt.Add(setupResendGap)); wait > 0 {
		return httpx.NewError(fiber.StatusTooManyRequests, "resend_too_soon",
			fmt.Sprintf("Please wait %d seconds before requesting another code.", int(wait/time.Second)+1))
	}
	code, err := secureCode()
	if err != nil {
		return httpx.Internal("could not generate a code")
	}
	if err := s.repo.ReplaceSetupCode(ctx, ch.ID, hashSetupCode(tokenHash, code), time.Now().Add(setupCodeTTL)); err != nil {
		return err
	}
	return s.sendSetupCode(ctx, ch.Email, ch.FullName, code)
}

// ── helpers ────────────────────────────────────────────────────────────────

func hashSetupCode(tokenHash, code string) string {
	sum := sha256.Sum256([]byte(tokenHash + ":" + code))
	return hex.EncodeToString(sum[:])
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// secureCode returns a uniformly random 6-digit code. Unlike GenerateOTPCode it
// never falls back to a fixed value: if the system RNG fails, it errors.
func secureCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// maskEmail turns "jane.doe@example.com" into "j*******@example.com".
func maskEmail(e string) string {
	at := strings.LastIndex(e, "@")
	if at <= 0 {
		return "your email"
	}
	local := e[:at]
	if len(local) <= 1 {
		return local + "***" + e[at:]
	}
	return local[:1] + strings.Repeat("*", min(len(local)-1, 7)) + e[at:]
}

// StaffPasswordError enforces the employee password policy and returns a
// human-readable reason, or "" when the password is acceptable. It is stricter
// than the seller policy: 10+ characters with all four character classes.
func StaffPasswordError(pw, email string) string {
	if len(pw) < 10 {
		return "Password must be at least 10 characters"
	}
	if len(pw) > 72 {
		return "Password must be 72 characters or fewer"
	}
	var lower, upper, digit, symbol bool
	for _, r := range pw {
		switch {
		case r >= 'a' && r <= 'z':
			lower = true
		case r >= 'A' && r <= 'Z':
			upper = true
		case r >= '0' && r <= '9':
			digit = true
		default:
			symbol = true
		}
	}
	if !(lower && upper && digit && symbol) {
		return "Use uppercase, lowercase, a number and a symbol"
	}
	if at := strings.Index(email, "@"); at >= 4 {
		if strings.Contains(strings.ToLower(pw), strings.ToLower(email[:at])) {
			return "Password must not contain your email name"
		}
	}
	return ""
}

// GenerateTempPassword returns a random temporary password that satisfies the
// staff password policy. Look-alike characters (0/O, 1/l/I) are left out so it
// can be typed from an email without mistakes.
func GenerateTempPassword() (string, error) {
	const (
		lower   = "abcdefghijkmnpqrstuvwxyz"
		upper   = "ABCDEFGHJKLMNPQRSTUVWXYZ"
		digits  = "23456789"
		symbols = "@#$%&*!?"
		all     = lower + upper + digits + symbols
		length  = 14
	)
	pick := func(set string) (byte, error) {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(set))))
		if err != nil {
			return 0, err
		}
		return set[n.Int64()], nil
	}
	out := make([]byte, 0, length)
	for _, set := range []string{lower, upper, digits, symbols} {
		c, err := pick(set)
		if err != nil {
			return "", err
		}
		out = append(out, c)
	}
	for len(out) < length {
		c, err := pick(all)
		if err != nil {
			return "", err
		}
		out = append(out, c)
	}
	// Fisher–Yates shuffle so the guaranteed characters are not at fixed spots.
	for i := len(out) - 1; i > 0; i-- {
		j, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return "", err
		}
		out[i], out[j.Int64()] = out[j.Int64()], out[i]
	}
	return string(out), nil
}
