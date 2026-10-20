package router

import (
	"github.com/gofiber/fiber/v2"

	"github.com/carlitos/backend/internal/auth"
	"github.com/carlitos/backend/internal/httpx"
	"github.com/carlitos/backend/internal/notifications"
	"github.com/carlitos/backend/internal/sellers"
)

// publicSellerStore returns an approved seller's public store profile by slug.
func publicSellerStore(c *fiber.Ctx, repo *sellers.Repository) error {
	s, err := repo.FindBySlug(c.Context(), c.Params("slug"))
	if err != nil {
		return httpx.NotFound("seller_not_found", "Store not found")
	}
	// Only expose public-safe fields.
	return httpx.OK(c, fiber.Map{
		"id":                s.ID,
		"store_name":        s.StoreName,
		"store_slug":        s.StoreSlug,
		"store_description": s.StoreDescription,
		"rating_avg":        s.RatingAvg,
		"rating_count":      s.RatingCount,
	})
}

// notificationsPassthrough lists the current user's notifications regardless of
// portal (shared "me" endpoint).
func notificationsPassthrough(c *fiber.Ctx, store *notifications.Store) error {
	p := auth.From(c)
	if p == nil {
		return httpx.Unauthorized("unauthenticated", "Authentication required")
	}
	items, err := store.ListForUser(c.Context(), p.UserID, 50, 0)
	if err != nil {
		return err
	}
	return httpx.OK(c, items)
}
