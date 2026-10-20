package notifications

import (
	"github.com/gofiber/fiber/v2"

	"github.com/carlitos/backend/internal/auth"
	"github.com/carlitos/backend/internal/httpx"
)

// Handler exposes in-app notification endpoints (any authenticated portal).
type Handler struct{ store *Store }

// NewHandler builds a notifications Handler.
func NewHandler(store *Store) *Handler { return &Handler{store: store} }

// Register mounts notification routes (auth enforced by caller).
func (h *Handler) Register(r fiber.Router) {
	r.Get("/notifications", h.list)
	r.Get("/notifications/unread-count", h.unread)
	r.Post("/notifications/:id/read", h.markRead)
}

func (h *Handler) list(c *fiber.Ctx) error {
	items, err := h.store.ListForUser(c.Context(), auth.From(c).UserID, 50, 0)
	if err != nil {
		return err
	}
	return httpx.OK(c, items)
}

func (h *Handler) unread(c *fiber.Ctx) error {
	n, err := h.store.UnreadCount(c.Context(), auth.From(c).UserID)
	if err != nil {
		return err
	}
	return httpx.OK(c, fiber.Map{"unread": n})
}

func (h *Handler) markRead(c *fiber.Ctx) error {
	if err := h.store.MarkRead(c.Context(), auth.From(c).UserID, c.Params("id")); err != nil {
		return err
	}
	return httpx.Message(c, "Marked as read")
}
