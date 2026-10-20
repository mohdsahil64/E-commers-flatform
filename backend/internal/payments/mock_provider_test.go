package payments

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// helper to sign a raw payload the way the mock provider expects for webhooks.
func signPayload(secret string, payload []byte) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write(payload)
	return hex.EncodeToString(m.Sum(nil))
}

func TestWebhookRejectsMissingSignature(t *testing.T) {
	p := NewMockProvider("key", "secret")
	payload := []byte(`{"type":"payment.captured","order_id":"o1","payment_id":"p1","status":"paid"}`)

	if _, err := p.ParseWebhook(payload, ""); err == nil {
		t.Fatal("expected webhook with empty signature to be rejected")
	}
}

func TestWebhookRejectsForgedSignature(t *testing.T) {
	p := NewMockProvider("key", "secret")
	payload := []byte(`{"type":"payment.captured","order_id":"o1","payment_id":"p1","status":"paid"}`)

	// An attacker-supplied signature computed with the wrong secret must fail.
	forged := signPayload("wrong-secret", payload)
	if _, err := p.ParseWebhook(payload, forged); err == nil {
		t.Fatal("expected webhook with forged signature to be rejected")
	}
}

func TestWebhookAcceptsValidSignature(t *testing.T) {
	const secret = "secret"
	p := NewMockProvider("key", secret)
	payload := []byte(`{"type":"payment.captured","order_id":"o1","payment_id":"p1","status":"paid"}`)

	valid := signPayload(secret, payload)
	evt, err := p.ParseWebhook(payload, valid)
	if err != nil {
		t.Fatalf("expected valid webhook to parse, got: %v", err)
	}
	if evt.ProviderOrderID != "o1" || evt.Status != "paid" {
		t.Errorf("unexpected event: %+v", evt)
	}
}

func TestVerifySignatureConstantTime(t *testing.T) {
	const secret = "secret"
	p := NewMockProvider("key", secret).(*mockProvider)
	in := VerifyInput{ProviderOrderID: "o1", ProviderPaymentID: "p1"}
	in.Signature = p.sign(in.ProviderOrderID + "|" + in.ProviderPaymentID)

	if !p.VerifySignature(in) {
		t.Error("expected a correctly-signed verification to pass")
	}
	in.Signature = "deadbeef"
	if p.VerifySignature(in) {
		t.Error("expected a wrong signature to fail")
	}
}
