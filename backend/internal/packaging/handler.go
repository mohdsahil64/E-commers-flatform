package packaging

import (
	"github.com/gofiber/fiber/v2"

	"github.com/carlitos/backend/internal/auth"
	"github.com/carlitos/backend/internal/httpx"
	"github.com/carlitos/backend/internal/sellers"
)

// Handler exposes packaging endpoints for seller and admin portals.
type Handler struct {
	svc          *Service
	sellerLookup *sellers.Repository
}

// NewHandler builds a packaging Handler.
func NewHandler(svc *Service, sellerLookup *sellers.Repository) *Handler {
	return &Handler{svc: svc, sellerLookup: sellerLookup}
}

// RegisterSeller mounts seller packaging routes.
func (h *Handler) RegisterSeller(r fiber.Router) {
	r.Get("/packaging/items", h.listItems)
	r.Get("/packaging/balances", h.balances)
	r.Get("/packaging/requests", h.myRequests)
	r.Post("/packaging/requests", h.createRequest)
	r.Post("/packaging/usage", h.recordUsage)
}

// RegisterAdmin mounts admin packaging routes.
func (h *Handler) RegisterAdmin(r fiber.Router) {
	r.Get("/packaging/items", auth.RequirePermission(auth.PermPackagingView), h.listItems)
	r.Post("/packaging/items", auth.RequirePermission(auth.PermPackagingManage), h.createItem)
	r.Post("/packaging/items/:id/stock", auth.RequirePermission(auth.PermPackagingManage), h.addStock)
	r.Get("/packaging/requests", auth.RequirePermission(auth.PermPackagingView), h.allRequests)
	r.Post("/packaging/requests/:id/approve", auth.RequirePermission(auth.PermPackagingManage), h.approve)
	r.Post("/packaging/requests/:id/reject", auth.RequirePermission(auth.PermPackagingManage), h.reject)
}

func (h *Handler) listItems(c *fiber.Ctx) error {
	items, err := h.svc.ListItems(c.Context())
	if err != nil {
		return err
	}
	return httpx.OK(c, items)
}

type createItemBody struct {
	SKU  string `json:"sku"`
	Name string `json:"name"`
	Type string `json:"type"`
	Unit string `json:"unit"`
}

func (h *Handler) createItem(c *fiber.Ctx) error {
	var b createItemBody
	if err := c.BodyParser(&b); err != nil {
		return httpx.BadRequest("invalid_body", "Invalid request body")
	}
	id, err := h.svc.CreateItem(c.Context(), b.SKU, b.Name, b.Type, b.Unit)
	if err != nil {
		return err
	}
	return httpx.Created(c, fiber.Map{"id": id})
}

type qtyBody struct {
	Quantity int `json:"quantity"`
}

func (h *Handler) addStock(c *fiber.Ctx) error {
	var b qtyBody
	if err := c.BodyParser(&b); err != nil {
		return httpx.BadRequest("invalid_body", "Invalid request body")
	}
	if err := h.svc.AddStock(c.Context(), c.Params("id"), b.Quantity, auth.From(c).UserID); err != nil {
		return err
	}
	return httpx.Message(c, "Stock added")
}

func (h *Handler) allRequests(c *fiber.Ctx) error {
	items, err := h.svc.ListRequests(c.Context(), "", c.Query("status"))
	if err != nil {
		return err
	}
	return httpx.OK(c, items)
}

type approveBody struct {
	ApprovedQuantity int `json:"approved_quantity"`
}

func (h *Handler) approve(c *fiber.Ctx) error {
	var b approveBody
	_ = c.BodyParser(&b)
	if err := h.svc.ApproveRequest(c.Context(), c.Params("id"), b.ApprovedQuantity, auth.From(c).UserID); err != nil {
		return err
	}
	return httpx.Message(c, "Request approved and packaging issued")
}

func (h *Handler) reject(c *fiber.Ctx) error {
	if err := h.svc.RejectRequest(c.Context(), c.Params("id"), auth.From(c).UserID); err != nil {
		return err
	}
	return httpx.Message(c, "Request rejected")
}

// ---- seller ----

func (h *Handler) balances(c *fiber.Ctx) error {
	sellerID, err := h.sellerID(c)
	if err != nil {
		return err
	}
	items, err := h.svc.SellerBalances(c.Context(), sellerID)
	if err != nil {
		return err
	}
	return httpx.OK(c, items)
}

func (h *Handler) myRequests(c *fiber.Ctx) error {
	sellerID, err := h.sellerID(c)
	if err != nil {
		return err
	}
	items, err := h.svc.ListRequests(c.Context(), sellerID, c.Query("status"))
	if err != nil {
		return err
	}
	return httpx.OK(c, items)
}

type reqBody struct {
	PackagingItemID string `json:"packaging_item_id"`
	Quantity        int    `json:"quantity"`
}

func (h *Handler) createRequest(c *fiber.Ctx) error {
	sellerID, err := h.sellerID(c)
	if err != nil {
		return err
	}
	var b reqBody
	if err := c.BodyParser(&b); err != nil || b.PackagingItemID == "" {
		return httpx.BadRequest("validation_failed", "packaging_item_id and quantity are required")
	}
	id, err := h.svc.CreateRequest(c.Context(), sellerID, b.PackagingItemID, b.Quantity)
	if err != nil {
		return err
	}
	return httpx.Created(c, fiber.Map{"id": id})
}

type usageBody struct {
	PackagingItemID string `json:"packaging_item_id"`
	Quantity        int    `json:"quantity"`
}

func (h *Handler) recordUsage(c *fiber.Ctx) error {
	sellerID, err := h.sellerID(c)
	if err != nil {
		return err
	}
	var b usageBody
	if err := c.BodyParser(&b); err != nil || b.PackagingItemID == "" {
		return httpx.BadRequest("validation_failed", "packaging_item_id and quantity are required")
	}
	if err := h.svc.RecordUsage(c.Context(), sellerID, b.PackagingItemID, b.Quantity); err != nil {
		return err
	}
	return httpx.Message(c, "Usage recorded")
}

func (h *Handler) sellerID(c *fiber.Ctx) (string, error) {
	seller, err := h.sellerLookup.FindByUserID(c.Context(), auth.From(c).UserID)
	if err != nil {
		return "", httpx.Forbidden("no_seller", "Seller profile not found")
	}
	return seller.ID, nil
}
