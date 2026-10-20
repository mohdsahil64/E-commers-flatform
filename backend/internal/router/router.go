package router

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/carlitos/backend/internal/app"
	"github.com/carlitos/backend/internal/httpx"
)

func Register(root fiber.Router, a *app.App) {
	root.Static("/static", "./static")

	// When Cloudinary is the active storage backend, media is served directly
	// from Cloudinary's CDN — the local /media route is unnecessary, so we skip
	// mounting it entirely.
	if strings.EqualFold(a.Config.Storage.Provider, "cloudinary") {
		registerAPI(root, a)
		return
	}

	// Serve uploaded media (product images, etc.) straight from local disk.
	// Fiber uses OS sendfile for speed, and a 30-day Cache-Control makes the
	// browser cache images so repeat loads across all three portals are instant.
	mediaRoot := a.Config.Storage.LocalRoot
	if mediaRoot == "" {
		mediaRoot = "./.data/storage"
	}

	// Security gate in front of the static handler:
	//  - only image extensions are servable, so no stray file in the storage
	//    dir (or a spoofed upload) can ever be downloaded as HTML/JS/etc.
	//  - nosniff stops browsers from MIME-sniffing a response into script.
	//  - inline disposition + no referrer, defence-in-depth against XSS.
	// Fiber's Static also cleans the path and refuses to serve outside the
	// root, which blocks "../" path-traversal attempts.
	allowedMediaExt := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".avif": true, ".gif": true,
		".mp4": true, ".webm": true, ".mov": true, ".m4v": true,
	}
	root.Use("/media", func(c *fiber.Ctx) error {
		if c.Method() != fiber.MethodGet && c.Method() != fiber.MethodHead {
			return httpx.Fail(c, fiber.StatusMethodNotAllowed, "method_not_allowed", "Only GET is allowed")
		}
		ext := strings.ToLower(filepath.Ext(c.Path()))
		if !allowedMediaExt[ext] {
			return httpx.Fail(c, fiber.StatusNotFound, "not_found", "Not found")
		}
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("Content-Disposition", "inline")
		c.Set("Referrer-Policy", "no-referrer")
		return c.Next()
	})
	root.Static("/media", mediaRoot, fiber.Static{
		Compress:      true,
		ByteRange:     true,
		MaxAge:        60 * 60 * 24 * 30, // 30 days (seconds)
		CacheDuration: 10 * time.Second,  // in-memory not-found cache
	})

	registerAPI(root, a)
}

// registerAPI mounts the versioned API groups and all feature modules. It is
// shared between the local-storage and Cloudinary paths of Register.
func registerAPI(root fiber.Router, a *app.App) {
	v1 := root.Group("/api/v1")
	v1.Get("/ping", func(c *fiber.Ctx) error {
		return httpx.OK(c, fiber.Map{"pong": true, "version": "v1"})
	})

	public := v1.Group("/public")
	seller := v1.Group("/seller")
	admin := v1.Group("/admin")
	auth := v1.Group("/auth")

	registerModules(a, v1, public, seller, admin, auth)
}
