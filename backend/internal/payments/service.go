package payments

import (
	"context"
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/carlitos/backend/internal/httpx"
)

// OrderConfirmer lets the payments service confirm an order once payment is
// captured, without importing the orders package (avoids an import cycle).
type OrderConfirmer interface {
	ConfirmPayment(ctx context.Context, orderID string) error
}

// Service ties the payment Provider to persistence and order confirmation.
type Service struct {
	pool      *pgxpool.Pool
	provider  Provider
	confirmer OrderConfirmer
	currency  string
}

// NewService builds a payments Service.
func NewService(pool *pgxpool.Pool, provider Provider, confirmer OrderConfirmer, currency string) *Service {
	if currency == "" {
		currency = "INR"
	}
	return &Service{pool: pool, provider: provider, confirmer: confirmer, currency: currency}
}

// InitiateForOrder creates a gateway payment order for an online order and
// persists a pending payment row. Returns the client-facing gateway handle.
func (s *Service) InitiateForOrder(ctx context.Context, orderID, userID string) (*ProviderOrder, error) {
	var amount float64
	var method, payStatus string
	err := s.pool.QueryRow(ctx, `
		SELECT o.grand_total, o.payment_method, o.payment_status
		FROM orders o JOIN customers cu ON cu.id = o.customer_id
		WHERE o.id = $1 AND cu.user_id = $2`, orderID, userID).
		Scan(&amount, &method, &payStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("order_not_found", "Order not found")
	}
	if err != nil {
		return nil, err
	}
	if method != "online" {
		return nil, httpx.BadRequest("not_online", "This order is not an online-payment order")
	}
	if payStatus == "paid" {
		return nil, httpx.Conflict("already_paid", "This order is already paid")
	}

	po, err := s.provider.CreateOrder(ctx, CreateOrderInput{
		OrderID: orderID, Amount: amount, Currency: s.currency, Receipt: orderID,
	})
	if err != nil {
		// Log the underlying gateway error internally; return a generic message
		// to the client so provider internals are never exposed.
		slog.Error("payment gateway CreateOrder failed", "order_id", orderID, "error", err)
		return nil, httpx.Internal("Payment could not be initiated. Please try again.")
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO payments (order_id, provider, method, amount, currency, status, provider_order_id)
		VALUES ($1,$2,'online',$3,$4,'pending',$5)`,
		orderID, s.provider.Name(), amount, s.currency, po.ProviderOrderID)
	if err != nil {
		return nil, err
	}
	return po, nil
}

// Verify validates a completed payment's signature, marks the payment paid, and
// confirms the order.
func (s *Service) Verify(ctx context.Context, in VerifyInput) error {
	if !s.provider.VerifySignature(in) {
		return httpx.BadRequest("invalid_signature", "Payment signature verification failed")
	}
	var orderID string
	err := s.pool.QueryRow(ctx,
		`SELECT order_id FROM payments WHERE provider_order_id = $1`, in.ProviderOrderID).Scan(&orderID)
	if errors.Is(err, pgx.ErrNoRows) {
		return httpx.NotFound("payment_not_found", "Payment not found")
	}
	if err != nil {
		return err
	}

	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			UPDATE payments SET status='paid', provider_payment_id=$2, provider_signature=$3
			WHERE provider_order_id=$1`, in.ProviderOrderID, in.ProviderPaymentID, in.Signature); err != nil {
			return err
		}
		var paymentID string
		if err := tx.QueryRow(ctx, `SELECT id FROM payments WHERE provider_order_id=$1`, in.ProviderOrderID).
			Scan(&paymentID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO payment_transactions (payment_id, event_type, status, provider_ref)
			VALUES ($1,'captured','paid',$2)`, paymentID, in.ProviderPaymentID)
		return err
	})
	if err != nil {
		return err
	}
	return s.confirmer.ConfirmPayment(ctx, orderID)
}

// MockComplete simulates a successful gateway callback for the development
// (mock) provider so the online-payment flow can be exercised end-to-end
// without a real gateway SDK. It generates a payment id + a valid signature
// server-side and runs the same Verify path (signature check, mark paid,
// confirm order). It is a no-op / error on any non-mock provider, so enabling a
// real provider (which returns a client-side signature) automatically disables
// this shortcut. The order is ownership-checked against the requesting user.
func (s *Service) MockComplete(ctx context.Context, orderID, userID string) error {
	if s.provider.Name() != "mock" {
		return httpx.BadRequest("not_supported", "Manual completion is only available with the mock gateway")
	}
	// Resolve the pending gateway order for this customer's order.
	var providerOrderID string
	err := s.pool.QueryRow(ctx, `
		SELECT pmt.provider_order_id
		FROM payments pmt
		JOIN orders o ON o.id = pmt.order_id
		JOIN customers cu ON cu.id = o.customer_id
		WHERE pmt.order_id = $1 AND cu.user_id = $2 AND pmt.status = 'pending'
		ORDER BY pmt.created_at DESC LIMIT 1`, orderID, userID).Scan(&providerOrderID)
	if errors.Is(err, pgx.ErrNoRows) {
		return httpx.NotFound("payment_not_found", "No pending payment found for this order")
	}
	if err != nil {
		return err
	}
	paymentID := "pay_mock_" + orderID
	signature := DevSignature("", providerOrderID, paymentID)
	return s.Verify(ctx, VerifyInput{
		ProviderOrderID:   providerOrderID,
		ProviderPaymentID: paymentID,
		Signature:         signature,
	})
}

// HandleWebhook processes an inbound gateway webhook.
func (s *Service) HandleWebhook(ctx context.Context, payload []byte, signature string) error {
	evt, err := s.provider.ParseWebhook(payload, signature)
	if err != nil {
		return httpx.BadRequest("invalid_webhook", "Invalid webhook payload")
	}
	if evt.ProviderOrderID == "" {
		return nil
	}
	var paymentID, orderID string
	if err := s.pool.QueryRow(ctx,
		`SELECT id, order_id FROM payments WHERE provider_order_id=$1`, evt.ProviderOrderID).
		Scan(&paymentID, &orderID); err != nil {
		return nil // unknown order: ack silently
	}
	_, _ = s.pool.Exec(ctx, `
		INSERT INTO payment_transactions (payment_id, event_type, status, provider_ref, raw_payload)
		VALUES ($1,'webhook',$2,$3,$4)`, paymentID, evt.Status, evt.ProviderPaymentID, evt.Raw)

	if evt.Status == "paid" || evt.Type == "payment.captured" {
		_, _ = s.pool.Exec(ctx, `UPDATE payments SET status='paid' WHERE id=$1`, paymentID)
		return s.confirmer.ConfirmPayment(ctx, orderID)
	}
	return nil
}
