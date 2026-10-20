package payments

import (
	"github.com/gofiber/fiber/v2"

	"github.com/carlitos/backend/internal/auth"
	"github.com/carlitos/backend/internal/httpx"
)

// Handler exposes payment endpoints.
type Handler struct{ svc *Service }

// NewHandler builds a payments Handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// RegisterCustomer mounts authenticated customer payment routes.
func (h *Handler) RegisterCustomer(r fiber.Router) {
	r.Post("/orders/:orderId/pay", h.initiate)
	r.Post("/payments/verify", h.verify)
	// Dev-only shortcut to complete a mock-gateway payment end-to-end (no-op on
	// real providers). Lets the customer portal finish the online flow without a
	// gateway SDK during development.
	r.Post("/orders/:orderId/pay/mock-complete", h.mockComplete)
}

// RegisterPublic mounts the unauthenticated webhook endpoint.
func (h *Handler) RegisterPublic(r fiber.Router) {
	r.Post("/payments/webhook", h.webhook)
}

func (h *Handler) initiate(c *fiber.Ctx) error {
	po, err := h.svc.InitiateForOrder(c.Context(), c.Params("orderId"), auth.From(c).UserID)
	if err != nil {
		return err
	}
	return httpx.OK(c, po)
}

type verifyBody struct {
	ProviderOrderID   string `json:"provider_order_id"`
	ProviderPaymentID string `json:"provider_payment_id"`
	Signature         string `json:"signature"`
}

func (h *Handler) verify(c *fiber.Ctx) error {
	var b verifyBody
	if err := c.BodyParser(&b); err != nil {
		return httpx.BadRequest("invalid_body", "Invalid request body")
	}
	if err := h.svc.Verify(c.Context(), VerifyInput{
		ProviderOrderID: b.ProviderOrderID, ProviderPaymentID: b.ProviderPaymentID, Signature: b.Signature,
	}); err != nil {
		return err
	}
	return httpx.Message(c, "Payment verified")
}

func (h *Handler) mockComplete(c *fiber.Ctx) error {
	if err := h.svc.MockComplete(c.Context(), c.Params("orderId"), auth.From(c).UserID); err != nil {
		return err
	}
	return httpx.Message(c, "Payment completed")
}

func (h *Handler) webhook(c *fiber.Ctx) error {
	// Razorpay signs webhooks with the "X-Razorpay-Signature" header; the mock
	// gateway uses "X-Webhook-Signature". Accept whichever is present so the
	// same endpoint serves both without a provider-specific route.
	sig := c.Get("X-Razorpay-Signature")
	if sig == "" {
		sig = c.Get("X-Webhook-Signature")
	}
	if err := h.svc.HandleWebhook(c.Context(), c.Body(), sig); err != nil {
		return err
	}
	return httpx.Message(c, "ok")
}
