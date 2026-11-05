package fulfillment

import (
	"strconv"
	"strings"

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
	r.Get("/shipments/ready", h.ready) // packed orders still waiting for a shipment (before :id)
	r.Post("/orders/:orderId/ship", h.createShipment)   // courier: create shipment + book pickup
	r.Post("/orders/:orderId/self-ship", h.selfShip)    // seller delivers themselves
	r.Get("/shipments/:id", h.sellerDetail)
	r.Post("/shipments/:id/mark-shipped", h.markShipped)
	r.Post("/shipments/:id/mark-delivered", h.markDelivered)
	r.Post("/shipments/:id/cancel", h.cancel)
	r.Post("/shipments/:id/refresh", h.sellerRefresh)
	r.Get("/shipments/:id/label", h.label)              // self-ship label PDF
	r.Get("/shipments/:id/track", h.track)
}

// RegisterAdmin mounts the admin Shipping page. It is read-only monitoring
// (order.view) plus a "refresh tracking" action (order.manage); the seller owns
// the physical hand-over, so admin cannot mark shipments shipped or delivered.
func (h *Handler) RegisterAdmin(r fiber.Router) {
	view := auth.RequireAnyPermission(auth.PermOrderView)
	manage := auth.RequirePermission(auth.PermOrderManage)
	r.Get("/shipments", view, h.adminList)
	r.Get("/shipments/summary", view, h.adminSummary) // before :id
	r.Get("/shipments/:id", view, h.adminDetail)
	r.Post("/shipments/:id/refresh", manage, h.adminRefresh)
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

func (h *Handler) ready(c *fiber.Ctx) error {
	sellerID, err := h.sellerID(c)
	if err != nil {
		return err
	}
	items, err := h.svc.ReadyToShip(c.Context(), sellerID)
	if err != nil {
		return err
	}
	return httpx.OK(c, items)
}

func (h *Handler) sellerDetail(c *fiber.Ctx) error {
	sellerID, err := h.sellerID(c)
	if err != nil {
		return err
	}
	d, err := h.svc.Detail(c.Context(), c.Params("id"), sellerID)
	if err != nil {
		return err
	}
	return httpx.OK(c, d)
}

func (h *Handler) cancel(c *fiber.Ctx) error {
	sellerID, err := h.sellerID(c)
	if err != nil {
		return err
	}
	if err := h.svc.Cancel(c.Context(), c.Params("id"), sellerID); err != nil {
		return err
	}
	return httpx.Message(c, "Shipment cancelled. You can book it again from Ready to ship.")
}

func (h *Handler) sellerRefresh(c *fiber.Ctx) error {
	sellerID, err := h.sellerID(c)
	if err != nil {
		return err
	}
	if err := h.svc.Refresh(c.Context(), c.Params("id"), sellerID); err != nil {
		return err
	}
	d, err := h.svc.Detail(c.Context(), c.Params("id"), sellerID)
	if err != nil {
		return err
	}
	return httpx.OK(c, d)
}

// ---- admin ----

func (h *Handler) adminList(c *fiber.Ctx) error {
	page := atoiDefault(c.Query("page"), 1)
	if page < 1 {
		page = 1
	}
	size := atoiDefault(c.Query("page_size"), 25)
	if size < 1 || size > 100 {
		size = 25
	}
	items, total, err := h.svc.ListAdmin(c.Context(), AdminFilter{
		Status:    strings.TrimSpace(c.Query("status")),
		ShipType:  strings.TrimSpace(c.Query("type")),
		Query:     strings.TrimSpace(c.Query("q")),
		Attention: c.Query("attention") == "1",
		Limit:     size,
		Offset:    (page - 1) * size,
	})
	if err != nil {
		return err
	}
	return httpx.Paginated(c, items, httpx.Meta{
		Page: page, PageSize: size, Total: total,
		TotalPages: int((total + int64(size) - 1) / int64(size)),
	})
}

func (h *Handler) adminSummary(c *fiber.Ctx) error {
	sm, err := h.svc.SummaryAdmin(c.Context())
	if err != nil {
		return err
	}
	return httpx.OK(c, sm)
}

func (h *Handler) adminDetail(c *fiber.Ctx) error {
	d, err := h.svc.Detail(c.Context(), c.Params("id"), "")
	if err != nil {
		return err
	}
	return httpx.OK(c, d)
}

func (h *Handler) adminRefresh(c *fiber.Ctx) error {
	if err := h.svc.Refresh(c.Context(), c.Params("id"), ""); err != nil {
		return err
	}
	d, err := h.svc.Detail(c.Context(), c.Params("id"), "")
	if err != nil {
		return err
	}
	return httpx.OK(c, d)
}

func atoiDefault(v string, def int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
		return n
	}
	return def
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
	sellerID, err := h.sellerID(c)
	if err != nil {
		return err
	}
	events, err := h.svc.Track(c.Context(), c.Params("id"), sellerID)
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
