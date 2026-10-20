package payments

import (
	"log/slog"
	"strings"

	"github.com/carlitos/backend/internal/config"
)

// NewProvider selects the configured payment provider based on PAYMENT_PROVIDER.
//
//   - "mock"     → development adapter, no credentials needed (not allowed in prod).
//   - "razorpay" → live Razorpay REST adapter. TEST vs LIVE is decided by which
//     API key pair you put in PAYMENT_KEY_ID / PAYMENT_KEY_SECRET
//     (rzp_test_* = Razorpay test mode, rzp_live_* = real money). The code path
//     is identical, so flipping test ↔ live is a .env-only change.
//
// The mock provider is intended for development only. The config layer already
// refuses to boot in production with PAYMENT_PROVIDER=mock (see
// config.validateProduction), so reaching the mock here implies a non-prod
// environment. Unknown providers log a warning and fall back to mock in dev.
func NewProvider(cfg config.PaymentConfig) Provider {
	switch strings.ToLower(cfg.Provider) {
	case "mock", "":
		return NewMockProvider(cfg.KeyID, cfg.KeySecret)
	case "razorpay":
		slog.Info("payment provider: razorpay",
			"mode", razorpayMode(cfg.KeyID))
		return NewRazorpayProvider(cfg.KeyID, cfg.KeySecret, cfg.WebhookSecret)
	// case "stripe":   return NewStripeProvider(cfg)    // future
	default:
		slog.Warn("unknown payment provider, using mock adapter (development only)",
			"provider", cfg.Provider)
		return NewMockProvider(cfg.KeyID, cfg.KeySecret)
	}
}

// razorpayMode reports "test" or "live" from the key id prefix, purely for
// startup logging so operators can confirm which Razorpay environment is active.
func razorpayMode(keyID string) string {
	switch {
	case strings.HasPrefix(keyID, "rzp_live_"):
		return "live"
	case strings.HasPrefix(keyID, "rzp_test_"):
		return "test"
	default:
		return "unknown (check PAYMENT_KEY_ID)"
	}
}
