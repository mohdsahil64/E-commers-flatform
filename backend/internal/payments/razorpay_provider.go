package payments

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// razorpayBaseURL is the Razorpay REST API root. The same host serves both test
// and live traffic — which environment you hit is decided purely by the API key
// pair (rzp_test_... vs rzp_live_...), so switching test ↔ live is a .env-only
// change with no code difference.
const razorpayBaseURL = "https://api.razorpay.com/v1"

// razorpayProvider is the production Razorpay adapter. It talks to the Razorpay
// REST API directly over HTTP (no third-party SDK) and implements the same
// Provider interface as the mock adapter, so order/checkout/settlement code is
// unchanged when PAYMENT_PROVIDER=razorpay.
//
// TEST vs LIVE: identical code path. Supplying rzp_test_* keys runs Razorpay in
// test mode (test cards/UPI, no real money); rzp_live_* keys run live. The
// adapter neither knows nor cares which — the toggle lives entirely in .env.
type razorpayProvider struct {
	keyID         string
	keySecret     string
	webhookSecret string
	http          *http.Client
}

// NewRazorpayProvider builds the Razorpay adapter. keyID/keySecret are the API
// key pair from the Razorpay dashboard (test or live). webhookSecret is the
// secret configured on the Razorpay webhook (used to verify inbound webhooks);
// if empty, webhook signature verification is skipped-safe (events ignored).
func NewRazorpayProvider(keyID, keySecret, webhookSecret string) Provider {
	return &razorpayProvider{
		keyID:         keyID,
		keySecret:     keySecret,
		webhookSecret: webhookSecret,
		http:          &http.Client{Timeout: 20 * time.Second},
	}
}

// CreateOrder creates a Razorpay order. Amount is sent in the smallest currency
// unit (paise for INR). The returned KeyID is the publishable key the client
// SDK (Razorpay Checkout) needs to open the payment popup.
func (p *razorpayProvider) CreateOrder(ctx context.Context, in CreateOrderInput) (*ProviderOrder, error) {
	amountMinor := int64(in.Amount*100 + 0.5)
	reqBody := map[string]any{
		"amount":          amountMinor,
		"currency":        in.Currency,
		"receipt":         in.Receipt,
		"payment_capture": 1, // auto-capture on successful authorization
	}
	var out struct {
		ID       string `json:"id"`
		Amount   int64  `json:"amount"`
		Currency string `json:"currency"`
	}
	if err := p.do(ctx, http.MethodPost, "/orders", reqBody, &out); err != nil {
		return nil, err
	}
	return &ProviderOrder{
		ProviderOrderID: out.ID,
		Amount:          out.Amount,
		Currency:        out.Currency,
		KeyID:           p.keyID,
	}, nil
}

// VerifySignature validates the signature returned by Razorpay Checkout on the
// client after a successful payment. Razorpay signs HMAC-SHA256(order_id +
// "|" + payment_id) with the API key secret.
func (p *razorpayProvider) VerifySignature(in VerifyInput) bool {
	if in.Signature == "" || in.ProviderOrderID == "" || in.ProviderPaymentID == "" {
		return false
	}
	expected := hmacSHA256Hex(p.keySecret, in.ProviderOrderID+"|"+in.ProviderPaymentID)
	return hmac.Equal([]byte(expected), []byte(in.Signature))
}

// Refund issues a (full or partial) refund against a captured payment.
func (p *razorpayProvider) Refund(ctx context.Context, in RefundInput) (*RefundResult, error) {
	body := map[string]any{}
	if in.Amount > 0 {
		body["amount"] = int64(in.Amount*100 + 0.5)
	}
	if in.Reason != "" {
		body["notes"] = map[string]string{"reason": in.Reason}
	}
	var out struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	path := "/payments/" + in.ProviderPaymentID + "/refund"
	if err := p.do(ctx, http.MethodPost, path, body, &out); err != nil {
		return nil, err
	}
	return &RefundResult{ProviderRefundID: out.ID, Status: out.Status}, nil
}

// ParseWebhook verifies the Razorpay webhook signature (X-Razorpay-Signature,
// HMAC-SHA256 of the raw body with the webhook secret) and normalizes the event.
// The signature arrives via the service as the `signature` argument.
func (p *razorpayProvider) ParseWebhook(payload []byte, signature string) (*WebhookEvent, error) {
	if p.webhookSecret == "" {
		return nil, errors.New("razorpay webhook secret not configured")
	}
	if signature == "" {
		return nil, errors.New("missing webhook signature")
	}
	expected := hmacSHA256Hex(p.webhookSecret, string(payload))
	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return nil, errors.New("invalid webhook signature")
	}

	// Razorpay webhook envelope: { event, payload: { payment: { entity: {...} } } }
	var env struct {
		Event   string `json:"event"`
		Payload struct {
			Payment struct {
				Entity struct {
					ID      string `json:"id"`
					OrderID string `json:"order_id"`
					Status  string `json:"status"`
				} `json:"entity"`
			} `json:"payment"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(payload, &env); err != nil {
		return nil, fmt.Errorf("parse razorpay webhook: %w", err)
	}
	entity := env.Payload.Payment.Entity
	// Normalize to the generic "paid" status the service understands.
	status := entity.Status
	if entity.Status == "captured" || env.Event == "payment.captured" {
		status = "paid"
	}
	return &WebhookEvent{
		Type:              env.Event,
		ProviderOrderID:   entity.OrderID,
		ProviderPaymentID: entity.ID,
		Status:            status,
		Raw:               payload,
	}, nil
}

func (p *razorpayProvider) Name() string { return "razorpay" }

// do performs an authenticated Razorpay REST call (HTTP Basic auth with the API
// key pair) and decodes the JSON response into out. On a non-2xx response it
// returns an error carrying Razorpay's error description.
func (p *razorpayProvider) do(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, razorpayBaseURL+path, reader)
	if err != nil {
		return err
	}
	req.SetBasicAuth(p.keyID, p.keySecret)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var e struct {
			Error struct {
				Code        string `json:"code"`
				Description string `json:"description"`
			} `json:"error"`
		}
		_ = json.Unmarshal(respBody, &e)
		desc := e.Error.Description
		if desc == "" {
			desc = strings.TrimSpace(string(respBody))
		}
		return fmt.Errorf("razorpay %s %s: status %d: %s", method, path, resp.StatusCode, desc)
	}
	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("decode razorpay response: %w", err)
		}
	}
	return nil
}

// hmacSHA256Hex computes a lowercase hex HMAC-SHA256, the encoding Razorpay uses
// for both payment and webhook signatures.
func hmacSHA256Hex(secret, msg string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(msg))
	return hex.EncodeToString(m.Sum(nil))
}
