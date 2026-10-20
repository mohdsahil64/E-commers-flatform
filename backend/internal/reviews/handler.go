package reviews

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/carlitos/backend/internal/auth"
	"github.com/carlitos/backend/internal/httpx"
)

// Handler exposes review endpoints.
type Handler struct{ svc *Service }

// NewHandler builds a reviews Handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// RegisterPublic mounts public review reads.
func (h *Handler) RegisterPublic(r fiber.Router) {
	r.Get("/products/:productId/reviews", h.listForProduct)
}

// RegisterCustomer mounts authenticated review creation.
func (h *Handler) RegisterCustomer(r fiber.Router) {
	r.Post("/reviews", h.create)
}

// RegisterAdmin mounts review moderation.
func (h *Handler) RegisterAdmin(r fiber.Router) {
	r.Get("/reviews", auth.RequirePermission(auth.PermReviewModerate), h.listModeration)
	r.Post("/reviews/:id/moderate", auth.RequirePermission(auth.PermReviewModerate), h.moderate)
}

func (h *Handler) listForProduct(c *fiber.Ctx) error {
	page := qi(c, "page", 1)
	size := qi(c, "page_size", 10)
	items, err := h.svc.ListForProduct(c.Context(), c.Params("productId"), size, (page-1)*size)
	if err != nil {
		return err
	}
	return httpx.OK(c, items)
}

type createBody struct {
	OrderItemID string   `json:"order_item_id"`
	Rating      int      `json:"rating"`
	Title       string   `json:"title"`
	Body        string   `json:"body"`
	PhotoKeys   []string `json:"photo_keys"`
}

func (h *Handler) create(c *fiber.Ctx) error {
	var b createBody
	if err := c.BodyParser(&b); err != nil || b.OrderItemID == "" {
		return httpx.BadRequest("validation_failed", "order_item_id and rating are required")
	}
	id, err := h.svc.Create(c.Context(), CreateInput{
		UserID: auth.From(c).UserID, OrderItemID: b.OrderItemID,
		Rating: b.Rating, Title: b.Title, Body: b.Body, PhotoKeys: b.PhotoKeys,
	})
	if err != nil {
		return err
	}
	return httpx.Created(c, fiber.Map{"id": id})
}

func (h *Handler) listModeration(c *fiber.Ctx) error {
	page := qi(c, "page", 1)
	size := qi(c, "page_size", 20)
	items, err := h.svc.ListForModeration(c.Context(), c.Query("status"), size, (page-1)*size)
	if err != nil {
		return err
	}
	return httpx.OK(c, items)
}

type moderateBody struct {
	Status string `json:"status"`
}

func (h *Handler) moderate(c *fiber.Ctx) error {
	var b moderateBody
	if err := c.BodyParser(&b); err != nil {
		return httpx.BadRequest("invalid_body", "Invalid request body")
	}
	if err := h.svc.Moderate(c.Context(), c.Params("id"), b.Status, auth.From(c).UserID); err != nil {
		return err
	}
	return httpx.Message(c, "Review moderated")
}

func qi(c *fiber.Ctx, key string, def int) int {
	if v := c.Query(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
