package customers

import (
	"context"
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/carlitos/backend/internal/auth"
	"github.com/carlitos/backend/internal/httpx"
)

// AuditRecorder is satisfied by *audit.Logger; declared here to keep this
// package free of an import on audit.
type AuditRecorder interface {
	Record(ctx context.Context, actorID, actorRole, action, entityType, entityID string, prev, next any, ip, ua string)
}

// RegisterAdmin mounts the admin customer-management routes. Viewing needs
// customer.view; blocking / unblocking needs customer.block.
func (h *Handler) RegisterAdmin(r fiber.Router, log AuditRecorder) {
	view := auth.RequirePermission(auth.PermCustomerView)
	block := auth.RequirePermission(auth.PermCustomerBlock)

	r.Get("/customers", view, h.adminList)
	r.Get("/customers/block-reasons", view, h.adminBlockReasons) // before :id
	r.Get("/customers/:id", view, h.adminGet)
	r.Post("/customers/:id/block", block, func(c *fiber.Ctx) error { return h.adminBlock(c, log) })
	r.Post("/customers/:id/unblock", block, func(c *fiber.Ctx) error { return h.adminUnblock(c, log) })
}

func qIntDefault(c *fiber.Ctx, key string, def int) int {
	if v, err := strconv.Atoi(c.Query(key)); err == nil && v > 0 {
		return v
	}
	return def
}

func (h *Handler) adminList(c *fiber.Ctx) error {
	page := qIntDefault(c, "page", 1)
	size := qIntDefault(c, "page_size", 20)
	if size > 100 {
		size = 100
	}
	items, total, err := h.svc.ListForAdmin(c.Context(), c.Query("status"), c.Query("q"), size, (page-1)*size)
	if err != nil {
		return err
	}
	return httpx.Paginated(c, items, httpx.Meta{Page: page, PageSize: size, Total: total,
		TotalPages: int((total + int64(size) - 1) / int64(size))})
}

func (h *Handler) adminBlockReasons(c *fiber.Ctx) error {
	return httpx.OK(c, BlockReasons)
}

func (h *Handler) adminGet(c *fiber.Ctx) error {
	d, err := h.svc.GetForAdmin(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return httpx.OK(c, d)
}

type blockBody struct {
	ReasonCode string `json:"reason_code"`
	Note       string `json:"note"`
}

func (h *Handler) adminBlock(c *fiber.Ctx, log AuditRecorder) error {
	var b blockBody
	if err := c.BodyParser(&b); err != nil {
		return httpx.BadRequest("invalid_body", "Invalid request body")
	}
	actor := auth.From(c).UserID
	res, err := h.svc.Block(c.Context(), c.Params("id"), actor, b.ReasonCode, b.Note)
	if err != nil {
		return err
	}
	if log != nil {
		log.Record(c.Context(), actor, "admin", "customer.blocked", "customer", c.Params("id"),
			map[string]any{"status": "active"}, map[string]any{"status": "blocked", "reason": res.Reason},
			c.IP(), c.Get("User-Agent"))
	}
	return httpx.Message(c, "Customer blocked")
}

func (h *Handler) adminUnblock(c *fiber.Ctx, log AuditRecorder) error {
	actor := auth.From(c).UserID
	if err := h.svc.Unblock(c.Context(), c.Params("id")); err != nil {
		return err
	}
	if log != nil {
		log.Record(c.Context(), actor, "admin", "customer.unblocked", "customer", c.Params("id"),
			map[string]any{"status": "blocked"}, map[string]any{"status": "active"},
			c.IP(), c.Get("User-Agent"))
	}
	return httpx.Message(c, "Customer unblocked")
}
