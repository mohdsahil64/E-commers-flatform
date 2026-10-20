// Package server builds the Fiber HTTP application: middleware stack, health
// checks, and delegates versioned route registration to the router package.
package server

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"

	"github.com/carlitos/backend/internal/app"
	"github.com/carlitos/backend/internal/httpx"
)

// Build constructs the Fiber app with all middleware and routes registered.
func Build(a *app.App, register func(router fiber.Router, a *app.App)) *fiber.App {
	f := fiber.New(fiber.Config{
		AppName:               a.Config.App.Name,
		ErrorHandler:          httpx.ErrorHandler,
		DisableStartupMessage: false,
		BodyLimit:             25 * 1024 * 1024, // 25 MB (product image uploads)
	})

	f.Use(requestid.New())
	f.Use(recover.New())
	f.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} ${method} ${path} (${latency})\n",
	}))
	// CORS: only enable credentialed CORS when explicit origins are configured.
	// A wildcard origin combined with AllowCredentials:true is invalid/insecure
	// (the browser rejects it and it would expose credentials), so when no
	// origins are set we fall back to a non-credentialed wildcard.
	corsCfg := cors.Config{
		AllowMethods: "GET,POST,PUT,PATCH,DELETE,OPTIONS",
		AllowHeaders: "Origin,Content-Type,Accept,Authorization,X-Requested-With",
	}
	if len(a.Config.CORS.AllowedOrigins) > 0 {
		corsCfg.AllowOrigins = joinOrigins(a.Config.CORS.AllowedOrigins)
		corsCfg.AllowCredentials = true
	} else {
		corsCfg.AllowOrigins = "*"
		corsCfg.AllowCredentials = false
	}
	f.Use(cors.New(corsCfg))

	// Liveness / readiness.
	f.Get("/health", func(c *fiber.Ctx) error {
		return httpx.OK(c, fiber.Map{"status": "ok", "app": a.Config.App.Name})
	})
	f.Get("/health/ready", func(c *fiber.Ctx) error {
		ctx := context.Background()
		dbOK := a.DB.Health(ctx) == nil
		redisOK := a.Redis == nil || a.Redis.Health(ctx) == nil
		status := fiber.StatusOK
		if !dbOK || !redisOK {
			status = fiber.StatusServiceUnavailable
		}
		return c.Status(status).JSON(fiber.Map{
			"success":  dbOK && redisOK,
			"database": dbOK,
			"redis":    redisOK,
		})
	})

	// Versioned API routes.
	if register != nil {
		register(f, a)
	}

	// 404 fallback.
	f.Use(func(c *fiber.Ctx) error {
		return httpx.Fail(c, fiber.StatusNotFound, "not_found", "Route not found")
	})

	return f
}

func joinOrigins(origins []string) string {
	if len(origins) == 0 {
		return "*"
	}
	out := origins[0]
	for _, o := range origins[1:] {
		out += "," + o
	}
	return out
}
