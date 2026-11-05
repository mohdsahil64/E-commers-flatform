package settlements

import (
	"github.com/gofiber/fiber/v2"

	"github.com/carlitos/backend/internal/auth"
	"github.com/carlitos/backend/internal/httpx"
	"github.com/carlitos/backend/internal/sellers"
)

// Handler exposes settlement endpoints.
type Handler struct {
	svc          *Service
	sellerLookup *sellers.Repository
}

// NewHandler builds a settlements Handler.
func NewHandler(svc *Service, sellerLookup *sellers.Repository) *Handler {
	return &Handler{svc: svc, sellerLookup: sellerLookup}
}

// RegisterSeller mounts seller settlement reads.
func (h *Handler) RegisterSeller(r fiber.Router) {
	r.Get("/settlements", h.mine)
	r.Get("/settlements/summary", h.summary) // must stay before /:id
	r.Get("/settlements/:id", h.sellerDetail)
}

// sellerDetail returns one of the caller's own settlements with its order lines.
func (h *Handler) sellerDetail(c *fiber.Ctx) error {
	seller, err := h.sellerLookup.FindByUserID(c.Context(), auth.From(c).UserID)
	if err != nil {
		return httpx.Forbidden("no_seller", "Seller profile not found")
	}
	d, err := h.svc.GetDetail(c.Context(), c.Params("id"), seller.ID)
	if err != nil {
		return err
	}
	return httpx.OK(c, d)
}

func (h *Handler) adminDetail(c *fiber.Ctx) error {
	d, err := h.svc.GetDetail(c.Context(), c.Params("id"), "")
	if err != nil {
		return err
	}
	return httpx.OK(c, d)
}

func (h *Handler) preview(c *fiber.Ctx) error {
	rows, err := h.svc.Preview(c.Context())
	if err != nil {
		return err
	}
	return httpx.OK(c, rows)
}

func (h *Handler) summary(c *fiber.Ctx) error {
	seller, err := h.sellerLookup.FindByUserID(c.Context(), auth.From(c).UserID)
	if err != nil {
		return httpx.Forbidden("no_seller", "Seller profile not found")
	}
	sum, err := h.svc.SummaryForSeller(c.Context(), seller.ID)
	if err != nil {
		return err
	}
	return httpx.OK(c, sum)
}

func (h *Handler) generateAll(c *fiber.Ctx) error {
	res, err := h.svc.GenerateAll(c.Context(), auth.From(c).UserID)
	if err != nil {
		return err
	}
	if res == nil {
		res = []GenerateResult{}
	}
	return httpx.OK(c, res)
}

// RegisterAdmin mounts admin settlement management.
func (h *Handler) RegisterAdmin(r fiber.Router) {
	r.Get("/settlements", auth.RequirePermission(auth.PermSettlementView), h.all)
	r.Get("/settlements/preview", auth.RequirePermission(auth.PermSettlementView), h.preview) // before /:id
	r.Get("/settlements/:id", auth.RequirePermission(auth.PermSettlementView), h.adminDetail)
	r.Post("/settlements/generate", auth.RequirePermission(auth.PermSettlementManage), h.generate)
	r.Post("/settlements/generate-all", auth.RequirePermission(auth.PermSettlementManage), h.generateAll)
	r.Post("/settlements/:id/status", auth.RequirePermission(auth.PermSettlementManage), h.status)
}

func (h *Handler) mine(c *fiber.Ctx) error {
	seller, err := h.sellerLookup.FindByUserID(c.Context(), auth.From(c).UserID)
	if err != nil {
		return httpx.Forbidden("no_seller", "Seller profile not found")
	}
	items, err := h.svc.ListForSeller(c.Context(), seller.ID)
	if err != nil {
		return err
	}
	return httpx.OK(c, items)
}

func (h *Handler) all(c *fiber.Ctx) error {
	items, err := h.svc.ListAll(c.Context(), c.Query("status"))
	if err != nil {
		return err
	}
	return httpx.OK(c, items)
}

type generateBody struct {
	SellerID string `json:"seller_id"`
}

func (h *Handler) generate(c *fiber.Ctx) error {
	var b generateBody
	if err := c.BodyParser(&b); err != nil || b.SellerID == "" {
		return httpx.BadRequest("validation_failed", "seller_id is required")
	}
	id, err := h.svc.Generate(c.Context(), b.SellerID, auth.From(c).UserID)
	if err != nil {
		return err
	}
	return httpx.Created(c, fiber.Map{"id": id})
}

type statusBody struct {
	Status string `json:"status"`
	UTR    string `json:"utr_reference"`
}

func (h *Handler) status(c *fiber.Ctx) error {
	var b statusBody
	if err := c.BodyParser(&b); err != nil {
		return httpx.BadRequest("invalid_body", "Invalid request body")
	}
	if err := h.svc.MarkStatus(c.Context(), auth.From(c).UserID, c.Params("id"), b.Status, b.UTR); err != nil {
		return err
	}
	return httpx.Message(c, "Settlement updated")
}
