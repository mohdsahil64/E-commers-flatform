package fulfillment

import (
	"github.com/gofiber/fiber/v2"

	"github.com/carlitos/backend/internal/auth"
	"github.com/carlitos/backend/internal/httpx"
	"github.com/carlitos/backend/internal/labelpdf"
	"github.com/carlitos/backend/internal/sellers"
)

// Handler exposes seller fulfilment (shipping) endpoints.
type Handler struct {
	svc          *Service
	sellerLookup *sellers.Repository
	webhookToken string
}

// NewHandler builds a fulfillment Handler.
func NewHandler(svc *Service, sellerLookup *sellers.Repository) *Handler {
	return &Handler{svc: svc, sellerLookup: sellerLookup}
}

// RegisterSeller mounts seller shipping routes.
func (h *Handler) RegisterSeller(r fiber.Router) {
	r.Get("/shipments", h.list)
	r.Post("/orders/:orderId/ship", h.createShipment)   // courier: create shipment + book pickup
	r.Post("/orders/:orderId/self-ship", h.selfShip)    // seller delivers themselves
	r.Post("/shipments/:id/mark-shipped", h.markShipped)
	r.Post("/shipments/:id/mark-delivered", h.markDelivered)
	r.Get("/shipments/:id/label", h.label)              // self-ship label PDF
	r.Get("/shipments/:id/track", h.track)
}

// RegisterCustomer mounts customer tracking routes (AWB resolved server-side).
func (h *Handler) RegisterCustomer(r fiber.Router) {
	r.Get("/orders/:id/track", h.customerTrack)
}

// RegisterPublic mounts the unauthenticated courier webhook.
func (h *Handler) RegisterPublic(r fiber.Router, webhookToken string) {
	h.webhookToken = webhookToken
	r.Post("/shipping/webhook", h.webhook)
}

func (h *Handler) list(c *fiber.Ctx) error {
	sellerID, err := h.sellerID(c)
	if err != nil {
		return err
	}
	items, err := h.svc.ListForSeller(c.Context(), sellerID)
	if err != nil {
		return err
	}
	return httpx.OK(c, items)
}

func (h *Handler) createShipment(c *fiber.Ctx) error {
	sellerID, err := h.sellerID(c)
	if err != nil {
		return err
	}
	shipment, err := h.svc.CreateForOrder(c.Context(), c.Params("orderId"), sellerID)
	if err != nil {
		return err
	}
	return httpx.Created(c, shipment)
}

func (h *Handler) selfShip(c *fiber.Ctx) error {
	sellerID, err := h.sellerID(c)
	if err != nil {
		return err
	}
	shipment, err := h.svc.SelfShip(c.Context(), c.Params("orderId"), sellerID)
	if err != nil {
		return err
	}
	return httpx.Created(c, shipment)
}

func (h *Handler) markShipped(c *fiber.Ctx) error {
	sellerID, err := h.sellerID(c)
	if err != nil {
		return err
	}
	if err := h.svc.MarkShipped(c.Context(), c.Params("id"), sellerID); err != nil {
		return err
	}
	return httpx.Message(c, "Shipment marked as shipped")
}

func (h *Handler) markDelivered(c *fiber.Ctx) error {
	sellerID, err := h.sellerID(c)
	if err != nil {
		return err
	}
	if err := h.svc.MarkDelivered(c.Context(), c.Params("id"), sellerID); err != nil {
		return err
	}
	return httpx.Message(c, "Shipment marked as delivered")
}

// label streams the self-ship shipping-label PDF for printing.
func (h *Handler) label(c *fiber.Ctx) error {
	sellerID, err := h.sellerID(c)
	if err != nil {
		return err
	}
	data, err := h.svc.SelfShipLabelData(c.Context(), c.Params("id"), sellerID)
	if err != nil {
		return err
	}
	pdf := labelpdf.Generate(*data)
	c.Set("Content-Type", "application/pdf")
	c.Set("Content-Disposition", `attachment; filename="carlitos-label-`+data.OrderNumber+`.pdf"`)
	return c.Send(pdf)
}

func (h *Handler) track(c *fiber.Ctx) error {
	events, err := h.svc.Track(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return httpx.OK(c, events)
}

// customerTrack returns the tracking timeline for the requesting customer's own
// order — no AWB entry required (resolved server-side, ownership-scoped).
func (h *Handler) customerTrack(c *fiber.Ctx) error {
	tracking, err := h.svc.TrackForCustomerUser(c.Context(), c.Params("id"), auth.From(c).UserID)
	if err != nil {
		return err
	}
	return httpx.OK(c, tracking)
}

// webhook receives courier tracking callbacks. It authenticates via a shared
// token header (configured with the courier). Shiprocket sends its configured
// token in the `x-api-key` header.
func (h *Handler) webhook(c *fiber.Ctx) error {
	if h.webhookToken != "" {
		token := c.Get("x-api-key")
		if token == "" {
			token = c.Get("X-Webhook-Token")
		}
		if token != h.webhookToken {
			return httpx.Unauthorized("invalid_webhook", "Invalid webhook token")
		}
	}
	// Shiprocket webhook payload (subset). Field names per their tracking webhook.
	var body struct {
		AWB            string `json:"awb"`
		CurrentStatus  string `json:"current_status"`
		ShipmentStatus string `json:"shipment_status"`
		Location       string `json:"location"`
		Activity       string `json:"activity"`
	}
	if err := c.BodyParser(&body); err != nil {
		return httpx.BadRequest("invalid_webhook", "Invalid webhook payload")
	}
	status := body.CurrentStatus
	if status == "" {
		status = body.ShipmentStatus
	}
	if err := h.svc.HandleWebhook(c.Context(), body.AWB, status, body.Location, body.Activity); err != nil {
		return err
	}
	return httpx.Message(c, "ok")
}

func (h *Handler) sellerID(c *fiber.Ctx) (string, error) {
	seller, err := h.sellerLookup.FindByUserID(c.Context(), auth.From(c).UserID)
	if err != nil {
		return "", httpx.Forbidden("no_seller", "Seller profile not found")
	}
	return seller.ID, nil
}
