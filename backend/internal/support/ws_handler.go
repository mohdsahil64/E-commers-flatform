package support

import (
	"net"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/carlitos/backend/internal/auth"
	"github.com/carlitos/backend/internal/httpx"
)

// RegisterWS mounts the real-time support socket:
//
//	GET /support/ws?token=<access JWT>   (WebSocket upgrade)
//
// Browsers cannot set an Authorization header on a WebSocket, so the access
// token travels in the query string. The same socket serves all three portals;
// what each one receives is decided by its verified identity (see Client.wants).
func (h *Handler) RegisterWS(r fiber.Router, tokens *auth.Manager, allowedOrigins []string, authRepo *auth.Repository) {
	r.Get("/support/ws", func(c *fiber.Ctx) error {
		if !strings.EqualFold(c.Get("Upgrade"), "websocket") ||
			!strings.Contains(strings.ToLower(c.Get("Connection")), "upgrade") ||
			c.Get("Sec-WebSocket-Key") == "" || c.Get("Sec-WebSocket-Version") != "13" {
			return c.Status(fiber.StatusUpgradeRequired).SendString("WebSocket upgrade required")
		}

		// Only the configured frontends may open the socket from a browser.
		if origin := c.Get("Origin"); origin != "" && len(allowedOrigins) > 0 {
			ok := false
			for _, o := range allowedOrigins {
				if strings.EqualFold(strings.TrimRight(o, "/"), strings.TrimRight(origin, "/")) {
					ok = true
					break
				}
			}
			if !ok {
				return httpx.Forbidden("bad_origin", "Origin not allowed")
			}
		}

		claims, err := tokens.ParseAccess(c.Query("token"))
		if err != nil {
			return httpx.Unauthorized("invalid_token", "Invalid or expired token")
		}

		var client *Client
		switch claims.AccountType {
		case "customer":
			client = NewClient("customer", claims.UserID, "")
		case "seller":
			seller, err := h.sellerLookup.FindByUserID(c.Context(), claims.UserID)
			if err != nil {
				return httpx.Forbidden("no_seller", "Seller profile not found")
			}
			client = NewClient("seller", claims.UserID, seller.ID)
		case "admin":
			// Use the account's LIVE state from the database, not the roles
			// baked into the (client-held) token: a disabled or demoted
			// employee must lose access immediately.
			roles, perms, ok, err := authRepo.LiveAdminAccess(c.Context(), claims.UserID)
			if err != nil {
				return err
			}
			if !ok {
				return httpx.Forbidden("account_inactive", "This account is not active")
			}
			p := &auth.Principal{Roles: roles, Permissions: perms}
			if !p.HasPermission(auth.PermSupportManage) {
				return httpx.Forbidden("forbidden", "You do not have permission: "+auth.PermSupportManage)
			}
			client = NewClient("admin", claims.UserID, "")
		default:
			return httpx.Forbidden("forbidden", "Unsupported account type")
		}

		c.Status(fiber.StatusSwitchingProtocols)
		c.Set("Upgrade", "websocket")
		c.Set("Connection", "Upgrade")
		c.Set("Sec-WebSocket-Accept", wsAccept(c.Get("Sec-WebSocket-Key")))

		hub := h.svc.Hub()
		// The Hijack handler runs after the 101 response is flushed and owns
		// the raw connection from then on.
		c.Context().Hijack(func(conn net.Conn) { hub.Serve(conn, client) })
		return nil
	})
}
