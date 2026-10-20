// Package payments provides a provider-agnostic payment abstraction plus a
// development (mock) adapter. Order logic depends only on the Provider
// interface, so Razorpay/Stripe can be integrated later without rewriting
// checkout or settlement code.
package payments

import "context"

// CreateOrderInput asks the gateway to create a payment order.
type CreateOrderInput struct {
	OrderID  string
	Amount   float64 // in major units (e.g. rupees)
	Currency string
	Receipt  string
}

// ProviderOrder is the gateway's order handle returned to the client.
type ProviderOrder struct {
	ProviderOrderID string `json:"provider_order_id"`
	Amount          int64  `json:"amount"` // in minor units (paise)
	Currency        string `json:"currency"`
	KeyID           string `json:"key_id"` // publishable key for the client SDK
}

// VerifyInput carries the signature material returned by the client after a
// successful payment, to be verified server-side.
type VerifyInput struct {
	ProviderOrderID   string
	ProviderPaymentID string
	Signature         string
}

// RefundInput requests a refund against a captured payment.
type RefundInput struct {
	ProviderPaymentID string
	Amount            float64
	Reason            string
}

// RefundResult is the gateway's refund handle.
type RefundResult struct {
	ProviderRefundID string
	Status           string
}

// WebhookEvent is a normalized gateway webhook.
type WebhookEvent struct {
	Type              string
	ProviderOrderID   string
	ProviderPaymentID string
	Status            string
	Raw               []byte
}

// Provider is the payment gateway abstraction.
type Provider interface {
	// CreateOrder creates a gateway payment order.
	CreateOrder(ctx context.Context, in CreateOrderInput) (*ProviderOrder, error)
	// VerifySignature validates a completed payment's signature.
	VerifySignature(in VerifyInput) bool
	// Refund issues a refund.
	Refund(ctx context.Context, in RefundInput) (*RefundResult, error)
	// ParseWebhook verifies and normalizes a webhook payload.
	ParseWebhook(payload []byte, signature string) (*WebhookEvent, error)
	// Name identifies the active provider.
	Name() string
}
