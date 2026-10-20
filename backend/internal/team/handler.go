package team

import (
	"github.com/gofiber/fiber/v2"

	"github.com/carlitos/backend/internal/auth"
	"github.com/carlitos/backend/internal/httpx"
)

// Handler exposes team roster endpoints.
type Handler struct{ repo *Repository }

// NewHandler builds a team Handler.
func NewHandler(repo *Repository) *Handler { return &Handler{repo: repo} }

// RegisterPublic mounts the storefront read.
func (h *Handler) RegisterPublic(r fiber.Router) {
	r.Get("/team", h.listTeam)
}

// RegisterAdmin mounts admin management (list all + edit name/photo). Reuses the
// banner-manage permission (team is the same merchandising/content capability).
func (h *Handler) RegisterAdmin(r fiber.Router) {
	r.Get("/team", auth.RequirePermission(auth.PermBannerManage), h.adminListTeam)
	r.Put("/team/:id", auth.RequirePermission(auth.PermBannerManage), h.updateTeam)
}

func (h *Handler) listTeam(c *fiber.Ctx) error {
	items, err := h.repo.ListActive(c.Context())
	if err != nil {
		return err
	}
	return httpx.OK(c, items)
}

func (h *Handler) adminListTeam(c *fiber.Ctx) error {
	items, err := h.repo.AdminList(c.Context())
	if err != nil {
		return err
	}
	return httpx.OK(c, items)
}

type updateTeamBody struct {
	Name     string `json:"name"`
	PhotoKey string `json:"photo_key"`
}

func (h *Handler) updateTeam(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return httpx.BadRequest("validation_failed", "Team member id is required")
	}
	var b updateTeamBody
	if err := c.BodyParser(&b); err != nil || b.Name == "" {
		return httpx.BadRequest("validation_failed", "Name is required")
	}
	// Only name + photo are editable; any other body fields are ignored.
	if err := h.repo.UpdateMember(c.Context(), id, b.Name, b.PhotoKey); err != nil {
		return err
	}
	return httpx.Message(c, "Team member updated")
}
