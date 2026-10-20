package cart

import (
	"github.com/gofiber/fiber/v2"

	"github.com/carlitos/backend/internal/auth"
	"github.com/carlitos/backend/internal/httpx"
)

// Handler exposes customer cart + wishlist endpoints.
type Handler struct {
	repo *Repository
}

// NewHandler builds a cart Handler.
func NewHandler(repo *Repository) *Handler { return &Handler{repo: repo} }

// Register mounts cart + wishlist routes (customer auth enforced by caller).
func (h *Handler) Register(r fiber.Router) {
	r.Get("/cart", h.list)
	r.Post("/cart/items", h.add)
	r.Put("/cart/items/:listingId", h.update)
	r.Delete("/cart/items/:listingId", h.remove)

	r.Get("/wishlist", h.wishlist)
	r.Post("/wishlist/items", h.addWishlist)
	r.Delete("/wishlist/items/:productId", h.removeWishlist)
}

func (h *Handler) list(c *fiber.Ctx) error {
	items, err := h.repo.List(c.Context(), auth.From(c).UserID)
	if err != nil {
		return err
	}
	var total float64
	for _, it := range items {
		total += it.LineTotal
	}
	return httpx.OK(c, fiber.Map{"items": items, "subtotal": total, "count": len(items)})
}

type addBody struct {
	SellerListingID string `json:"seller_listing_id"`
	Quantity        int    `json:"quantity"`
}

func (h *Handler) add(c *fiber.Ctx) error {
	var b addBody
	if err := c.BodyParser(&b); err != nil || b.SellerListingID == "" {
		return httpx.BadRequest("validation_failed", "seller_listing_id is required")
	}
	if err := h.repo.AddItem(c.Context(), auth.From(c).UserID, b.SellerListingID, b.Quantity); err != nil {
		return err
	}
	return httpx.Message(c, "Added to cart")
}

type qtyBody struct {
	Quantity int `json:"quantity"`
}

func (h *Handler) update(c *fiber.Ctx) error {
	var b qtyBody
	if err := c.BodyParser(&b); err != nil {
		return httpx.BadRequest("invalid_body", "Invalid request body")
	}
	if err := h.repo.UpdateQuantity(c.Context(), auth.From(c).UserID, c.Params("listingId"), b.Quantity); err != nil {
		return err
	}
	return httpx.Message(c, "Cart updated")
}

func (h *Handler) remove(c *fiber.Ctx) error {
	if err := h.repo.RemoveItem(c.Context(), auth.From(c).UserID, c.Params("listingId")); err != nil {
		return err
	}
	return httpx.Message(c, "Removed from cart")
}

func (h *Handler) wishlist(c *fiber.Ctx) error {
	items, err := h.repo.ListWishlist(c.Context(), auth.From(c).UserID)
	if err != nil {
		return err
	}
	return httpx.OK(c, items)
}

type wishlistBody struct {
	ProductID string `json:"product_id"`
}

func (h *Handler) addWishlist(c *fiber.Ctx) error {
	var b wishlistBody
	if err := c.BodyParser(&b); err != nil || b.ProductID == "" {
		return httpx.BadRequest("validation_failed", "product_id is required")
	}
	if err := h.repo.AddWishlist(c.Context(), auth.From(c).UserID, b.ProductID); err != nil {
		return err
	}
	return httpx.Message(c, "Added to wishlist")
}

func (h *Handler) removeWishlist(c *fiber.Ctx) error {
	if err := h.repo.RemoveWishlist(c.Context(), auth.From(c).UserID, c.Params("productId")); err != nil {
		return err
	}
	return httpx.Message(c, "Removed from wishlist")
}
