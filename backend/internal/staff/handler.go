package staff

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"

	"github.com/carlitos/backend/internal/auth"
	"github.com/carlitos/backend/internal/httpx"
)

// Handler exposes the employee, role and own-account endpoints. It is mounted
// on the admin group, so auth.AdminGuard has already re-validated the caller's
// live account and permissions; each route below additionally declares the
// permission it needs.
type Handler struct{ svc *Service }

// NewHandler builds a staff Handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// RegisterAdmin mounts the routes.
func (h *Handler) RegisterAdmin(r fiber.Router) {
	// Invitation e-mails are rate-limited per signed-in admin so the endpoint
	// can't be used to flood an inbox.
	mailLimit := limiter.New(limiter.Config{
		Max:        15,
		Expiration: 10 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			if p := auth.From(c); p != nil {
				return "staff-mail:" + p.UserID
			}
			return "staff-mail:" + c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return httpx.Fail(c, fiber.StatusTooManyRequests, "rate_limited",
				"Too many invitations sent. Please wait a few minutes.")
		},
	})

	manageStaff := auth.RequirePermission(auth.PermUserManage)
	manageRoles := auth.RequirePermission(auth.PermRoleManage)
	viewRoles := auth.RequireAnyPermission(auth.PermUserManage, auth.PermRoleManage)

	r.Get("/staff", manageStaff, h.list)
	r.Post("/staff/invite", manageStaff, mailLimit, h.invite)
	r.Post("/staff/:id/resend", manageStaff, mailLimit, h.resend)
	r.Post("/staff/:id/disable", manageStaff, h.disable)
	r.Post("/staff/:id/enable", manageStaff, h.enable)
	r.Put("/staff/:id/role", manageStaff, h.changeRole)
	r.Delete("/staff/:id", manageStaff, h.cancelInvite)

	r.Get("/roles", viewRoles, h.listRoles)
	// The permission catalog (labels + what the caller holds) carries nothing
	// sensitive, and every admin needs it to read their own access on Settings.
	r.Get("/permissions", h.permissions)
	r.Post("/roles", manageRoles, h.createRole)
	r.Put("/roles/:id", manageRoles, h.updateRole)
	r.Delete("/roles/:id", manageRoles, h.deleteRole)

	// Own account: available to every signed-in admin.
	r.Get("/account", h.account)
	r.Put("/account", h.updateAccount)
	r.Post("/account/change-password", h.changePassword)
}

func actor(c *fiber.Ctx) (*auth.Principal, error) {
	p := auth.From(c)
	if p == nil {
		return nil, httpx.Unauthorized("unauthenticated", "Authentication required")
	}
	return p, nil
}

// ── employees ──

func (h *Handler) list(c *fiber.Ctx) error {
	p, err := actor(c)
	if err != nil {
		return err
	}
	items, err := h.svc.ListEmployees(c.Context(), p)
	if err != nil {
		return err
	}
	return httpx.OK(c, items)
}

type inviteBody struct {
	FullName string `json:"full_name"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	RoleID   string `json:"role_id"`
}

func (h *Handler) invite(c *fiber.Ctx) error {
	p, err := actor(c)
	if err != nil {
		return err
	}
	var b inviteBody
	if err := c.BodyParser(&b); err != nil {
		return httpx.BadRequest("invalid_body", "Invalid request body")
	}
	emp, emailed, err := h.svc.Invite(c.Context(), p, InviteInput{
		FullName: b.FullName, Email: b.Email, Phone: b.Phone, RoleID: b.RoleID,
	})
	if err != nil {
		return err
	}
	return httpx.Created(c, fiber.Map{"employee": emp, "email_sent": emailed})
}

func (h *Handler) resend(c *fiber.Ctx) error {
	p, err := actor(c)
	if err != nil {
		return err
	}
	emp, emailed, err := h.svc.ResendInvite(c.Context(), p, c.Params("id"))
	if err != nil {
		return err
	}
	return httpx.OK(c, fiber.Map{"employee": emp, "email_sent": emailed})
}

func (h *Handler) disable(c *fiber.Ctx) error {
	p, err := actor(c)
	if err != nil {
		return err
	}
	emp, err := h.svc.Disable(c.Context(), p, c.Params("id"))
	if err != nil {
		return err
	}
	return httpx.OK(c, emp)
}

func (h *Handler) enable(c *fiber.Ctx) error {
	p, err := actor(c)
	if err != nil {
		return err
	}
	emp, err := h.svc.Enable(c.Context(), p, c.Params("id"))
	if err != nil {
		return err
	}
	return httpx.OK(c, emp)
}

type roleIDBody struct {
	RoleID string `json:"role_id"`
}

func (h *Handler) changeRole(c *fiber.Ctx) error {
	p, err := actor(c)
	if err != nil {
		return err
	}
	var b roleIDBody
	if err := c.BodyParser(&b); err != nil {
		return httpx.BadRequest("invalid_body", "Invalid request body")
	}
	emp, err := h.svc.ChangeRole(c.Context(), p, c.Params("id"), b.RoleID)
	if err != nil {
		return err
	}
	return httpx.OK(c, emp)
}

func (h *Handler) cancelInvite(c *fiber.Ctx) error {
	p, err := actor(c)
	if err != nil {
		return err
	}
	if err := h.svc.CancelInvite(c.Context(), p, c.Params("id")); err != nil {
		return err
	}
	return httpx.Message(c, "Invitation removed")
}

// ── roles ──

func (h *Handler) listRoles(c *fiber.Ctx) error {
	p, err := actor(c)
	if err != nil {
		return err
	}
	roles, err := h.svc.ListRoles(c.Context(), p)
	if err != nil {
		return err
	}
	return httpx.OK(c, roles)
}

func (h *Handler) permissions(c *fiber.Ctx) error {
	p, err := actor(c)
	if err != nil {
		return err
	}
	return httpx.OK(c, h.svc.Permissions(p))
}

type roleBody struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

func (h *Handler) createRole(c *fiber.Ctx) error {
	p, err := actor(c)
	if err != nil {
		return err
	}
	var b roleBody
	if err := c.BodyParser(&b); err != nil {
		return httpx.BadRequest("invalid_body", "Invalid request body")
	}
	role, err := h.svc.CreateRole(c.Context(), p, b.Name, b.Description, b.Permissions)
	if err != nil {
		return err
	}
	return httpx.Created(c, role)
}

func (h *Handler) updateRole(c *fiber.Ctx) error {
	p, err := actor(c)
	if err != nil {
		return err
	}
	var b roleBody
	if err := c.BodyParser(&b); err != nil {
		return httpx.BadRequest("invalid_body", "Invalid request body")
	}
	role, err := h.svc.UpdateRole(c.Context(), p, c.Params("id"), b.Name, b.Description, b.Permissions)
	if err != nil {
		return err
	}
	return httpx.OK(c, role)
}

func (h *Handler) deleteRole(c *fiber.Ctx) error {
	p, err := actor(c)
	if err != nil {
		return err
	}
	if err := h.svc.DeleteRole(c.Context(), p, c.Params("id")); err != nil {
		return err
	}
	return httpx.Message(c, "Role deleted")
}

// ── own account ──

func (h *Handler) account(c *fiber.Ctx) error {
	p, err := actor(c)
	if err != nil {
		return err
	}
	acc, err := h.svc.Account(c.Context(), p)
	if err != nil {
		return err
	}
	return httpx.OK(c, acc)
}

type accountBody struct {
	FullName string `json:"full_name"`
	Phone    string `json:"phone"`
}

func (h *Handler) updateAccount(c *fiber.Ctx) error {
	p, err := actor(c)
	if err != nil {
		return err
	}
	var b accountBody
	if err := c.BodyParser(&b); err != nil {
		return httpx.BadRequest("invalid_body", "Invalid request body")
	}
	acc, err := h.svc.UpdateAccount(c.Context(), p, b.FullName, b.Phone)
	if err != nil {
		return err
	}
	return httpx.OK(c, acc)
}

type passwordBody struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (h *Handler) changePassword(c *fiber.Ctx) error {
	p, err := actor(c)
	if err != nil {
		return err
	}
	var b passwordBody
	if err := c.BodyParser(&b); err != nil {
		return httpx.BadRequest("invalid_body", "Invalid request body")
	}
	if b.CurrentPassword == "" || b.NewPassword == "" {
		return httpx.BadRequest("validation_failed", "Current and new password are required")
	}
	if err := h.svc.ChangePassword(c.Context(), p, b.CurrentPassword, b.NewPassword); err != nil {
		return err
	}
	return httpx.Message(c, "Password changed. Please sign in again.")
}
