package auth

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/carlitos/backend/internal/httpx"
)

// context keys used to store the authenticated principal on the Fiber ctx.
const (
	ctxClaims = "auth_claims"
)

// Principal is the resolved identity attached to a request.
type Principal struct {
	UserID      string
	AccountType string
	Email       string
	Roles       []string
	Permissions []string
}

// HasPermission reports whether the principal holds a permission. SUPER_ADMIN
// implicitly holds all permissions.
func (p *Principal) HasPermission(perm string) bool {
	for _, r := range p.Roles {
		if r == RoleSuperAdmin {
			return true
		}
	}
	for _, pm := range p.Permissions {
		if pm == perm {
			return true
		}
	}
	return false
}

// HasRole reports whether the principal has a given role.
func (p *Principal) HasRole(role string) bool {
	for _, r := range p.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// Middleware returns a Fiber handler that requires a valid access token and
// attaches the Principal to the context. accountType, if non-empty, enforces
// that the token belongs to that portal (customer|seller|admin).
func Middleware(m *Manager, accountType string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		raw := extractBearer(c)
		if raw == "" {
			return httpx.Unauthorized("missing_token", "Authorization token required")
		}
		claims, err := m.ParseAccess(raw)
		if err != nil {
			return httpx.Unauthorized("invalid_token", "Invalid or expired token")
		}
		if accountType != "" && claims.AccountType != accountType {
			return httpx.Forbidden("wrong_portal", "Token not valid for this portal")
		}
		c.Locals(ctxClaims, &Principal{
			UserID:      claims.UserID,
			AccountType: claims.AccountType,
			Email:       claims.Email,
			Roles:       claims.Roles,
			Permissions: claims.Permissions,
		})
		return c.Next()
	}
}

// RequirePermission ensures the principal holds all listed permissions.
func RequirePermission(perms ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		p := From(c)
		if p == nil {
			return httpx.Unauthorized("unauthenticated", "Authentication required")
		}
		for _, perm := range perms {
			if !p.HasPermission(perm) {
				return httpx.Forbidden("forbidden", "You do not have permission: "+perm)
			}
		}
		return c.Next()
	}
}

// RequireRole ensures the principal has at least one of the listed roles.
func RequireRole(roles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		p := From(c)
		if p == nil {
			return httpx.Unauthorized("unauthenticated", "Authentication required")
		}
		for _, role := range roles {
			if p.HasRole(role) {
				return c.Next()
			}
		}
		return httpx.Forbidden("forbidden", "Insufficient role")
	}
}

// From returns the Principal attached to the context, or nil.
func From(c *fiber.Ctx) *Principal {
	if v, ok := c.Locals(ctxClaims).(*Principal); ok {
		return v
	}
	return nil
}

func extractBearer(c *fiber.Ctx) string {
	h := c.Get("Authorization")
	if h == "" {
		return ""
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return strings.TrimSpace(parts[1])
	}
	return strings.TrimSpace(h)
}
