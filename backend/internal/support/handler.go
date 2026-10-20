package support

import (
	"github.com/gofiber/fiber/v2"

	"github.com/carlitos/backend/internal/auth"
	"github.com/carlitos/backend/internal/httpx"
	"github.com/carlitos/backend/internal/sellers"
)

// Handler exposes support endpoints across portals.
type Handler struct {
	svc          *Service
	sellerLookup *sellers.Repository
}

// NewHandler builds a support Handler.
func NewHandler(svc *Service, sellerLookup *sellers.Repository) *Handler {
	return &Handler{svc: svc, sellerLookup: sellerLookup}
}

// RegisterCustomer mounts customer support routes.
func (h *Handler) RegisterCustomer(r fiber.Router) {
	r.Get("/support/tickets", h.customerList)
	r.Post("/support/tickets", h.open)
	r.Post("/support/tickets/:id/messages", h.customerReply)
}

// RegisterSeller mounts seller support routes. The seller replies to and
// resolves only its own product tickets.
func (h *Handler) RegisterSeller(r fiber.Router) {
	r.Get("/support/tickets", h.sellerList)
	r.Post("/support/tickets/:id/messages", h.sellerReply)
	r.Post("/support/tickets/:id/resolve", h.sellerResolve)
}

// RegisterAdmin mounts admin support routes behind the support.manage
// permission. Admin handles non-product tickets and can read product tickets
// (scope=all) but never act on them (enforced in the service).
func (h *Handler) RegisterAdmin(r fiber.Router) {
	r.Get("/support/tickets", auth.RequirePermission(auth.PermSupportManage), h.adminList)
	r.Post("/support/tickets/:id/messages", auth.RequirePermission(auth.PermSupportManage), h.adminReply)
	r.Post("/support/tickets/:id/resolve", auth.RequirePermission(auth.PermSupportManage), h.adminResolve)
}

// ---- customer ----

type openBody struct {
	Category    string   `json:"category"`
	Subject     string   `json:"subject"`
	Description string   `json:"description"`
	OrderItemID string   `json:"order_item_id"`
	PhotoKeys   []string `json:"photo_keys"`
}

func (h *Handler) open(c *fiber.Ctx) error {
	var b openBody
	if err := c.BodyParser(&b); err != nil {
		return httpx.BadRequest("validation_failed", "Invalid request body")
	}
	id, err := h.svc.Open(c.Context(), OpenInput{
		UserID:      auth.From(c).UserID,
		Category:    b.Category,
		Subject:     b.Subject,
		Description: b.Description,
		OrderItemID: b.OrderItemID,
		PhotoKeys:   b.PhotoKeys,
	})
	if err != nil {
		return err
	}
	return httpx.Created(c, fiber.Map{"id": id})
}

func (h *Handler) customerList(c *fiber.Ctx) error {
	items, err := h.svc.ListForCustomer(c.Context(), auth.From(c).UserID)
	if err != nil {
		return err
	}
	return httpx.OK(c, items)
}

type replyBody struct {
	Body string `json:"body"`
}

func (h *Handler) customerReply(c *fiber.Ctx) error {
	var b replyBody
	_ = c.BodyParser(&b)
	if err := h.svc.Reply(c.Context(), ReplyInput{
		TicketID: c.Params("id"), UserID: auth.From(c).UserID,
		AuthorRole: "customer", Body: b.Body,
	}); err != nil {
		return err
	}
	return httpx.Message(c, "Reply sent")
}

// ---- seller ----

func (h *Handler) sellerFromCtx(c *fiber.Ctx) (string, error) {
	seller, err := h.sellerLookup.FindByUserID(c.Context(), auth.From(c).UserID)
	if err != nil {
		return "", httpx.Forbidden("no_seller", "Seller profile not found")
	}
	return seller.ID, nil
}

func (h *Handler) sellerList(c *fiber.Ctx) error {
	sellerID, err := h.sellerFromCtx(c)
	if err != nil {
		return err
	}
	items, err := h.svc.ListForSeller(c.Context(), sellerID)
	if err != nil {
		return err
	}
	return httpx.OK(c, items)
}

func (h *Handler) sellerReply(c *fiber.Ctx) error {
	sellerID, err := h.sellerFromCtx(c)
	if err != nil {
		return err
	}
	var b replyBody
	_ = c.BodyParser(&b)
	if err := h.svc.Reply(c.Context(), ReplyInput{
		TicketID: c.Params("id"), UserID: auth.From(c).UserID,
		AuthorRole: "seller", Body: b.Body, SellerID: sellerID,
	}); err != nil {
		return err
	}
	return httpx.Message(c, "Reply sent")
}

type resolveBody struct {
	Resolution string `json:"resolution"`
}

func (h *Handler) sellerResolve(c *fiber.Ctx) error {
	sellerID, err := h.sellerFromCtx(c)
	if err != nil {
		return err
	}
	var b resolveBody
	_ = c.BodyParser(&b)
	if err := h.svc.Resolve(c.Context(), ResolveInput{
		TicketID: c.Params("id"), UserID: auth.From(c).UserID,
		AuthorRole: "seller", Resolution: b.Resolution, SellerID: sellerID,
	}); err != nil {
		return err
	}
	return httpx.Message(c, "Ticket marked solved")
}

// ---- admin ----

func (h *Handler) adminList(c *fiber.Ctx) error {
	items, err := h.svc.ListForAdmin(c.Context(), c.Query("scope"), c.Query("status"))
	if err != nil {
		return err
	}
	return httpx.OK(c, items)
}

func (h *Handler) adminReply(c *fiber.Ctx) error {
	var b replyBody
	_ = c.BodyParser(&b)
	if err := h.svc.Reply(c.Context(), ReplyInput{
		TicketID: c.Params("id"), UserID: auth.From(c).UserID,
		AuthorRole: "admin", Body: b.Body,
	}); err != nil {
		return err
	}
	return httpx.Message(c, "Reply sent")
}

func (h *Handler) adminResolve(c *fiber.Ctx) error {
	var b resolveBody
	_ = c.BodyParser(&b)
	if err := h.svc.Resolve(c.Context(), ResolveInput{
		TicketID: c.Params("id"), UserID: auth.From(c).UserID,
		AuthorRole: "admin", Resolution: b.Resolution,
	}); err != nil {
		return err
	}
	return httpx.Message(c, "Ticket marked solved")
}
