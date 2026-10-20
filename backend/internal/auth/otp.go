package auth

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// OTPSender delivers a one-time code to a phone number. The default
// development implementation logs the code and accepts a fixed dev code, so the
// full mobile-login flow works without a real SMS gateway. A production SMS
// adapter can implement this interface and be swapped in at wiring time.
type OTPSender interface {
	// Send delivers the code to the phone (best-effort; returns an error only on
	// hard failures). Implementations must not block the request for long.
	Send(ctx context.Context, phone, code string) error
	// Name identifies the active sender (for diagnostics).
	Name() string
}

// DevOTPCode is the fixed code accepted in development so testers can log in
// with any phone number using 123456. Replace the sender in production.
const DevOTPCode = "123456"

// logOTPSender is the development sender: it logs the code instead of sending
// an SMS. Combined with the dev store below, any phone can verify with 123456.
type logOTPSender struct{ logger *slog.Logger }

// NewLogOTPSender builds the development OTP sender.
func NewLogOTPSender(logger *slog.Logger) OTPSender {
	if logger == nil {
		logger = slog.Default()
	}
	return &logOTPSender{logger: logger}
}

func (s *logOTPSender) Send(_ context.Context, phone, code string) error {
	s.logger.Info("OTP (development)", "phone", phone, "code", code)
	return nil
}

func (s *logOTPSender) Name() string { return "log" }

// otpEntry is a stored challenge for a phone number.
type otpEntry struct {
	code      string
	expiresAt time.Time
	attempts  int
}

// OTPStore keeps issued OTP challenges. This in-memory implementation is fine
// for a single-instance dev/staging deployment; a Redis-backed store can
// replace it for horizontal scaling (the interface is intentionally small).
type OTPStore struct {
	mu  sync.Mutex
	m   map[string]otpEntry
	ttl time.Duration
}

// NewOTPStore builds an in-memory OTP store with the given challenge TTL.
func NewOTPStore(ttl time.Duration) *OTPStore {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &OTPStore{m: make(map[string]otpEntry), ttl: ttl}
}

// Put stores a code for a phone, replacing any previous challenge.
func (s *OTPStore) Put(phone, code string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[phone] = otpEntry{code: code, expiresAt: time.Now().Add(s.ttl)}
}

// Verify checks a submitted code. It enforces expiry and a small attempt cap,
// and consumes the challenge on success. In development, the fixed DevOTPCode
// is always accepted (even if no challenge was issued) so testing is friction
// free; production senders should not use this store's dev acceptance.
func (s *OTPStore) Verify(phone, code string) bool {
	if code == DevOTPCode {
		s.mu.Lock()
		delete(s.m, phone)
		s.mu.Unlock()
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.m[phone]
	if !ok || time.Now().After(e.expiresAt) || e.attempts >= 5 {
		return false
	}
	if e.code != code {
		e.attempts++
		s.m[phone] = e
		return false
	}
	delete(s.m, phone)
	return true
}

// MarkVerified records that a key (e.g. an email or phone) successfully passed
// verification, keeping the marker for the store's TTL. Used to bridge a
// two-phase flow: verify the code first, then complete registration.
func (s *OTPStore) MarkVerified(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m["verified:"+key] = otpEntry{code: "1", expiresAt: time.Now().Add(s.ttl)}
}

// IsVerified reports whether a key was recently marked verified (within TTL). It
// does not consume the marker so both email and phone can be checked at submit.
func (s *OTPStore) IsVerified(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.m["verified:"+key]
	return ok && time.Now().Before(e.expiresAt)
}

// ClearVerified removes a verification marker (called after successful signup).
func (s *OTPStore) ClearVerified(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, "verified:"+key)
}
