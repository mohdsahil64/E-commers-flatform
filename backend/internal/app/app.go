// Package app wires together the application's long-lived dependencies
// (config, database, redis, providers) into a single container that the
// command entrypoints (api, migrate, seed) build and share.
package app

import (
	"context"
	"log/slog"

	"github.com/carlitos/backend/internal/applog"
	"github.com/carlitos/backend/internal/config"
	"github.com/carlitos/backend/internal/database"
	"github.com/carlitos/backend/internal/redisclient"
)

// App holds shared, long-lived dependencies.
type App struct {
	Config *config.Config
	Logger *slog.Logger
	DB     *database.DB
	Redis  *redisclient.Client
}

// New builds the application container: loads config and connects to Postgres
// and Redis. Callers must call Close when finished.
func New(ctx context.Context) (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	logger := applog.New(cfg.App.LogLevel, cfg.App.Env)

	db, err := database.Connect(ctx, cfg.DB.URL)
	if err != nil {
		return nil, err
	}

	rdb, err := redisclient.Connect(ctx, cfg.Redis.URL)
	if err != nil {
		db.Close()
		return nil, err
	}

	return &App{
		Config: cfg,
		Logger: logger,
		DB:     db,
		Redis:  rdb,
	}, nil
}

// NewWithoutRedis builds the container but skips Redis (used by tooling such as
// the migrate command that does not need a cache connection).
func NewWithoutRedis(ctx context.Context) (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	logger := applog.New(cfg.App.LogLevel, cfg.App.Env)
	db, err := database.Connect(ctx, cfg.DB.URL)
	if err != nil {
		return nil, err
	}
	return &App{Config: cfg, Logger: logger, DB: db}, nil
}

// Close releases all held resources.
func (a *App) Close() {
	if a.Redis != nil {
		_ = a.Redis.Close()
	}
	if a.DB != nil {
		a.DB.Close()
	}
}
