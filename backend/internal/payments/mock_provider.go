package payments

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// mockProvider is a development payment adapter. It simulates a gateway with
// deterministic IDs and HMAC signatures so the full online-payment flow can be
// exercised end-to-end without real credentials.
type mockProvider struct {
	keyID  string
	secret string
}

// NewMockProvider builds the development payment adapter.
func NewMockProvider(keyID, secret string) Provider {
	if keyID == "" {
		keyID = "carlitos_mock_key"
	}
	if secret == "" {
		secret = "carlitos_mock_secret"
	}
	return &mockProvider{keyID: keyID, secret: secret}
}

func (p *mockProvider) CreateOrder(ctx context.Context, in CreateOrderInput) (*ProviderOrder, error) {
	return &ProviderOrder{
		ProviderOrderID: "order_" + uuid.NewString(),
		Amount:          int64(in.Amount*100 + 0.5),
		Currency:        in.Currency,
		KeyID:           p.keyID,
	}, nil
}

// VerifySignature mimics Razorpay's HMAC(order_id|payment_id, secret) scheme.
func (p *mockProvider) VerifySignature(in VerifyInput) bool {
	expected := p.sign(in.ProviderOrderID + "|" + in.ProviderPaymentID)
	return hmac.Equal([]byte(expected), []byte(in.Signature))
}

// Sign is exposed for tests/dev tooling to compute a valid signature.
func (p *mockProvider) sign(msg string) string {
	m := hmac.New(sha256.New, []byte(p.secret))
	m.Write([]byte(msg))
	return hex.EncodeToString(m.Sum(nil))
}

func (p *mockProvider) Refund(ctx context.Context, in RefundInput) (*RefundResult, error) {
	return &RefundResult{ProviderRefundID: "rfnd_" + uuid.NewString(), Status: "processed"}, nil
}

// ParseWebhook verifies the webhook signature before trusting the payload. The
// signature is an HMAC-SHA256 of the raw request body using the provider secret
// (mirrors how Razorpay/Stripe sign webhooks). An unsigned or mismatched
// payload is rejected so the public webhook endpoint cannot be used to forge a
// "paid" event and confirm an order without a real payment.
func (p *mockProvider) ParseWebhook(payload []byte, signature string) (*WebhookEvent, error) {
	if signature == "" {
		return nil, errors.New("missing webhook signature")
	}
	expected := p.signBytes(payload)
	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return nil, errors.New("invalid webhook signature")
	}
	var body struct {
		Type      string `json:"type"`
		OrderID   string `json:"order_id"`
		PaymentID string `json:"payment_id"`
		Status    string `json:"status"`
	}
	if err := json.Unmarshal(payload, &body); err != nil {
		return nil, fmt.Errorf("parse webhook: %w", err)
	}
	return &WebhookEvent{
		Type: body.Type, ProviderOrderID: body.OrderID,
		ProviderPaymentID: body.PaymentID, Status: body.Status, Raw: payload,
	}, nil
}

// signBytes computes the HMAC-SHA256 of a raw byte payload (used for webhooks).
func (p *mockProvider) signBytes(payload []byte) string {
	m := hmac.New(sha256.New, []byte(p.secret))
	m.Write(payload)
	return hex.EncodeToString(m.Sum(nil))
}

func (p *mockProvider) Name() string { return "mock" }

// DevSignature computes a valid signature for the mock provider (development
// helper so the frontend can simulate a successful payment callback).
func DevSignature(secret, orderID, paymentID string) string {
	if secret == "" {
		secret = "carlitos_mock_secret"
	}
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(orderID + "|" + paymentID))
	return hex.EncodeToString(m.Sum(nil))
}

var _ = time.Now
