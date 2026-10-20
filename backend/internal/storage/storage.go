// Package storage provides a provider-agnostic object storage abstraction.
// Media is stored on the local filesystem and served by the API's /media
// route, so the rest of the application never depends on a specific provider.
package storage

import (
	"context"
	"io"
)

// Object describes a stored object's metadata.
type Object struct {
	Key         string
	Size        int64
	ContentType string
	URL         string
}

// MediaKind distinguishes image vs video media. It lets remote providers like
// Cloudinary pick the correct resource type / delivery path. Empty is treated
// as image for backward compatibility.
type MediaKind string

const (
	KindImage MediaKind = "image"
	KindVideo MediaKind = "video"
)

// UploadInput carries the data and metadata for an upload.
type UploadInput struct {
	Key         string
	ContentType string
	Body        io.Reader
	Size        int64
	// Kind selects image vs video handling (defaults to image when empty).
	Kind MediaKind
}

// Provider is the storage abstraction. The active implementation is the local
// filesystem adapter, whose files are served by the API's /media static route.
type Provider interface {
	// Upload stores an object and returns its metadata.
	Upload(ctx context.Context, in UploadInput) (*Object, error)
	// Delete removes an object by key.
	Delete(ctx context.Context, key string) error
	// PublicURL returns a stable public URL for a key.
	PublicURL(key string) string
	// Root returns the on-disk directory media is stored in.
	Root() string
	// Name identifies the active provider (for diagnostics).
	Name() string
}
