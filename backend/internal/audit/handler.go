package audit

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/carlitos/backend/internal/auth"
	"github.com/carlitos/backend/internal/httpx"
)

// Handler exposes admin audit-log reads.
type Handler struct{ logger *Logger }

// NewHandler builds an audit Handler.
func NewHandler(logger *Logger) *Handler { return &Handler{logger: logger} }

// RegisterAdmin mounts audit routes.
func (h *Handler) RegisterAdmin(r fiber.Router) {
	r.Get("/audit-logs", auth.RequirePermission(auth.PermAuditView), h.list)
}

func (h *Handler) list(c *fiber.Ctx) error {
	page := 1
	size := 50
	if v := c.Query("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			page = n
		}
	}
	entries, err := h.logger.List(c.Context(), size, (page-1)*size)
	if err != nil {
		return err
	}
	return httpx.OK(c, entries)
}
