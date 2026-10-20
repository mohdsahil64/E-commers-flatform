// Command api starts the CARLITOS HTTP server.
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/carlitos/backend/internal/app"
	"github.com/carlitos/backend/internal/router"
	"github.com/carlitos/backend/internal/server"
)

func main() {
	ctx := context.Background()

	a, err := app.New(ctx)
	if err != nil {
		panic(err)
	}
	defer a.Close()

	f := server.Build(a, router.Register)

	// Start server in a goroutine so we can handle graceful shutdown.
	go func() {
		addr := ":" + a.Config.App.Port
		a.Logger.Info("starting CARLITOS API", "addr", addr, "env", a.Config.App.Env)
		if err := f.Listen(addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.Logger.Error("server stopped", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	a.Logger.Info("shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := f.ShutdownWithContext(shutdownCtx); err != nil {
		a.Logger.Error("graceful shutdown failed", "error", err)
	}
	a.Logger.Info("bye")
}
