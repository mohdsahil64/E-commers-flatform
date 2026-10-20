package coupons

import (
	"github.com/gofiber/fiber/v2"

	"github.com/carlitos/backend/internal/auth"
	"github.com/carlitos/backend/internal/httpx"
)

// Handler exposes coupon endpoints for admin management and customer preview.
type Handler struct {
	svc               *Service
	resolveCustomerID func(c *fiber.Ctx) (string, error)
}

// NewHandler builds a coupon Handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// RegisterAdmin mounts admin coupon management (permission-gated).
func (h *Handler) RegisterAdmin(r fiber.Router) {
	g := r.Group("", auth.RequirePermission(auth.PermCouponManage))
	g.Get("/coupons", h.list)
	g.Post("/coupons", h.create)
	g.Put("/coupons/:id", h.update)
	g.Post("/coupons/:id/active", h.setActive)
	g.Delete("/coupons/:id", h.remove)
	g.Get("/coupons/redemptions", h.allRedemptions)
	g.Get("/coupons/:id/redemptions", h.couponRedemptions)
}

// RegisterCustomer mounts the authenticated customer coupon preview endpoint.
func (h *Handler) RegisterCustomer(r fiber.Router, resolveCustomerID func(ctx *fiber.Ctx) (string, error)) {
	h.resolveCustomerID = resolveCustomerID
	r.Get("/coupons", h.available)
	r.Post("/coupons/preview", h.preview)
}

// --- admin ---

func (h *Handler) list(c *fiber.Ctx) error {
	items, err := h.svc.Repo().List(c.Context())
	if err != nil {
		return err
	}
	return httpx.OK(c, items)
}

func (h *Handler) create(c *fiber.Ctx) error {
	var in CreateInput
	if err := c.BodyParser(&in); err != nil {
		return httpx.BadRequest("invalid_body", "Invalid request body")
	}
	if err := validateInput(in); err != nil {
		return err
	}
	id, err := h.svc.Repo().Create(c.Context(), in)
	if err != nil {
		return err
	}
	return httpx.Created(c, fiber.Map{"id": id})
}

func (h *Handler) update(c *fiber.Ctx) error {
	var in CreateInput
	if err := c.BodyParser(&in); err != nil {
		return httpx.BadRequest("invalid_body", "Invalid request body")
	}
	if err := validateInput(in); err != nil {
		return err
	}
	if err := h.svc.Repo().Update(c.Context(), c.Params("id"), in); err != nil {
		return err
	}
	return httpx.Message(c, "Coupon updated")
}

type activeBody struct {
	Active bool `json:"active"`
}

func (h *Handler) setActive(c *fiber.Ctx) error {
	var b activeBody
	if err := c.BodyParser(&b); err != nil {
		return httpx.BadRequest("invalid_body", "Invalid request body")
	}
	if err := h.svc.Repo().SetActive(c.Context(), c.Params("id"), b.Active); err != nil {
		return err
	}
	return httpx.Message(c, "Coupon updated")
}

func (h *Handler) remove(c *fiber.Ctx) error {
	if err := h.svc.Repo().Delete(c.Context(), c.Params("id")); err != nil {
		return err
	}
	return httpx.Message(c, "Coupon deleted")
}

func (h *Handler) allRedemptions(c *fiber.Ctx) error {
	items, err := h.svc.Repo().Redemptions(c.Context(), "")
	if err != nil {
		return err
	}
	return httpx.OK(c, items)
}

func (h *Handler) couponRedemptions(c *fiber.Ctx) error {
	items, err := h.svc.Repo().Redemptions(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return httpx.OK(c, items)
}

// --- customer ---

// available lists the active, currently-valid coupons a customer can see on the
// checkout "offers" screen. Eligibility against the current cart is computed on
// the client from min_order_amount; the discount is always re-validated and
// recomputed server-side at preview/checkout time.
func (h *Handler) available(c *fiber.Ctx) error {
	items, err := h.svc.Repo().ListAvailable(c.Context())
	if err != nil {
		return err
	}
	return httpx.OK(c, items)
}

type previewBody struct {
	Code     string  `json:"code"`
	Subtotal float64 `json:"subtotal"`
}

func (h *Handler) preview(c *fiber.Ctx) error {
	var b previewBody
	if err := c.BodyParser(&b); err != nil {
		return httpx.BadRequest("invalid_body", "Invalid request body")
	}
	customerID := ""
	if h.resolveCustomerID != nil {
		if id, err := h.resolveCustomerID(c); err == nil {
			customerID = id
		}
	}
	q, err := h.svc.Preview(c.Context(), b.Code, b.Subtotal, customerID)
	if err != nil {
		return err
	}
	return httpx.OK(c, q)
}

func validateInput(in CreateInput) error {
	if len(in.Code) < 3 {
		return httpx.BadRequest("validation_failed", "Coupon code must be at least 3 characters")
	}
	if in.DiscountType != "percent" && in.DiscountType != "fixed" {
		return httpx.BadRequest("validation_failed", "Discount type must be 'percent' or 'fixed'")
	}
	if in.DiscountValue <= 0 {
		return httpx.BadRequest("validation_failed", "Discount value must be greater than zero")
	}
	if in.DiscountType == "percent" && in.DiscountValue > 100 {
		return httpx.BadRequest("validation_failed", "Percent discount cannot exceed 100")
	}
	return nil
}
