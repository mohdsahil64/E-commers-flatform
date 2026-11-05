package auth

import (
	"context"
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"

	"github.com/carlitos/backend/internal/httpx"
)

// adminAccess is the LIVE (database) state of an admin account.
type adminAccess struct {
	accountType        string
	status             string
	mustChangePassword bool
}

// LiveAdminAccess returns the CURRENT roles and permissions of an admin account
// read straight from the database, and ok=false when the account is missing,
// not an admin account, disabled, or still has its first-login setup pending.
// Non-HTTP entry points (e.g. the support WebSocket) use it so they never trust
// the permissions embedded in a client-held token.
func (r *Repository) LiveAdminAccess(ctx context.Context, userID string) (roles, perms []string, ok bool, err error) {
	acc, err := r.loadAdminAccess(ctx, userID)
	if errors.Is(err, ErrNotFound) {
		return nil, nil, false, nil
	}
	if err != nil {
		return nil, nil, false, err
	}
	if acc.accountType != "admin" || acc.status != "active" || acc.mustChangePassword {
		return nil, nil, false, nil
	}
	roles, perms, err = r.LoadRolesAndPermissions(ctx, userID)
	if err != nil {
		return nil, nil, false, err
	}
	return roles, perms, true, nil
}

func (r *Repository) loadAdminAccess(ctx context.Context, userID string) (*adminAccess, error) {
	var a adminAccess
	err := r.pool.QueryRow(ctx, `
		SELECT u.account_type, u.status, coalesce(s.must_change_password, FALSE)
		FROM users u
		LEFT JOIN admin_staff s ON s.user_id = u.id
		WHERE u.id = $1`, userID).Scan(&a.accountType, &a.status, &a.mustChangePassword)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// AdminGuard is the server-side authority for every admin API call. It runs
// after the token middleware and does NOT trust the roles/permissions that are
// embedded in the (client-held) access token. On every request it re-reads the
// account from the database and:
//
//   - rejects the call if the account was disabled, removed or is not an admin
//     account (so revoking an employee takes effect immediately, not when the
//     access token eventually expires);
//   - rejects the call while the first-login password change is still pending;
//   - replaces the principal's roles and permissions with the CURRENT ones, so
//     a role change applies on the very next request.
//
// Together with RequirePermission on each route this means that editing the
// request, the token or the browser's stored data can never widen access.
func AdminGuard(repo *Repository) fiber.Handler {
	return func(c *fiber.Ctx) error {
		p := From(c)
		if p == nil {
			return httpx.Unauthorized("unauthenticated", "Authentication required")
		}
		acc, err := repo.loadAdminAccess(c.Context(), p.UserID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return httpx.Unauthorized("invalid_token", "Invalid or expired token")
			}
			return err
		}
		if acc.accountType != "admin" {
			return httpx.Forbidden("wrong_portal", "Token not valid for this portal")
		}
		if acc.status != "active" {
			return httpx.Forbidden("account_inactive", "This account has been disabled")
		}
		if acc.mustChangePassword {
			return httpx.Forbidden("password_change_required", "Finish your account setup to continue")
		}
		roles, perms, err := repo.LoadRolesAndPermissions(c.Context(), p.UserID)
		if err != nil {
			return err
		}
		c.Locals(ctxClaims, &Principal{
			UserID:      p.UserID,
			AccountType: p.AccountType,
			Email:       p.Email,
			Roles:       roles,
			Permissions: perms,
		})
		return c.Next()
	}
}

// RequireAnyPermission ensures the principal holds at least one of the listed
// permissions.
func RequireAnyPermission(perms ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		p := From(c)
		if p == nil {
			return httpx.Unauthorized("unauthenticated", "Authentication required")
		}
		for _, perm := range perms {
			if p.HasPermission(perm) {
				return c.Next()
			}
		}
		return httpx.Forbidden("forbidden", "You do not have permission to do this")
	}
}

// IsSuperAdmin reports whether the principal is a SUPER_ADMIN.
func (p *Principal) IsSuperAdmin() bool { return p.HasRole(RoleSuperAdmin) }

// HoldsAll reports whether the principal holds every permission in perms
// (SUPER_ADMIN holds everything). Used to stop privilege escalation: nobody can
// hand out a permission they do not have themselves.
func (p *Principal) HoldsAll(perms []string) bool {
	if p.IsSuperAdmin() {
		return true
	}
	for _, want := range perms {
		if !p.HasPermission(want) {
			return false
		}
	}
	return true
}
