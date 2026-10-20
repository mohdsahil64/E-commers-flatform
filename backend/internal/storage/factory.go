package storage

import (
	"strings"

	"github.com/carlitos/backend/internal/config"
)

// New constructs the configured storage provider.
//
//   - "cloudinary": uploads to Cloudinary and serves from its global CDN (fast
//     loads, no local disk). Chosen automatically when Cloudinary credentials
//     are present (see config.resolveStorageConfig).
//   - "local" (default): stores on the server's disk, served by the API's
//     /media static route.
func New(cfg config.StorageConfig) (Provider, error) {
	switch strings.ToLower(cfg.Provider) {
	case "cloudinary":
		return newCloudinaryProvider(cfg)
	default:
		return newLocalProvider(cfg)
	}
}
