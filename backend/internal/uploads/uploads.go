// Package uploads exposes a generic authenticated file-upload endpoint backed
// by the storage provider abstraction. It returns the stored object's key and
// public URL so the client can attach it to a product/review/etc.
package uploads

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/google/uuid"

	"github.com/carlitos/backend/internal/auth"
	"github.com/carlitos/backend/internal/httpx"
	"github.com/carlitos/backend/internal/storage"
)

// Handler exposes the upload endpoint.
type Handler struct {
	provider storage.Provider
}

// NewHandler builds an upload Handler.
func NewHandler(provider storage.Provider) *Handler { return &Handler{provider: provider} }

// allowedImageExt is the set of accepted image extensions.
var allowedImageExt = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".avif": true, ".gif": true,
}

// allowedVideoExt is the set of accepted video extensions (used by review /
// return media where customers may attach a short clip).
var allowedVideoExt = map[string]bool{
	".mp4": true, ".webm": true, ".mov": true, ".m4v": true,
}

// Size caps differ by media kind — videos are allowed to be larger than images.
const (
	maxImageBytes = 10 * 1024 * 1024  // 10 MB
	maxVideoBytes = 100 * 1024 * 1024 // 100 MB
)

// Register mounts the upload route (auth enforced by the caller's group).
// A per-IP rate limit caps how fast a single authenticated client can upload,
// so a compromised/abusive account can't rapidly fill the server's disk.
func (h *Handler) Register(r fiber.Router) {
	r.Post("/uploads", limiter.New(limiter.Config{
		Max:        30,
		Expiration: time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			if p := auth.From(c); p != nil && p.UserID != "" {
				return p.UserID
			}
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return httpx.Fail(c, fiber.StatusTooManyRequests, "rate_limited",
				"Too many uploads. Please slow down and try again shortly.")
		},
	}), h.upload)
}

func (h *Handler) upload(c *fiber.Ctx) error {
	if h.provider == nil {
		return httpx.Internal("storage is not configured")
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return httpx.BadRequest("no_file", "A 'file' form field is required")
	}
	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	isImage := allowedImageExt[ext]
	isVideo := allowedVideoExt[ext]
	if !isImage && !isVideo {
		return httpx.BadRequest("unsupported_type", "Only image or video files are allowed")
	}

	// Per-kind size limit: images stay small, videos may be larger.
	maxBytes := int64(maxImageBytes)
	if isVideo {
		maxBytes = maxVideoBytes
	}
	if fileHeader.Size > maxBytes {
		if isVideo {
			return httpx.BadRequest("too_large", "Video exceeds the 100MB limit")
		}
		return httpx.BadRequest("too_large", "File exceeds the 10MB limit")
	}

	f, err := fileHeader.Open()
	if err != nil {
		return httpx.Internal("failed to read upload")
	}
	defer f.Close()

	// Verify the actual bytes match the claimed kind, not just a trusted
	// extension. This blocks a script/HTML file renamed to .jpg/.mp4 (a
	// stored-XSS vector).
	head := make([]byte, 512)
	n, _ := f.Read(head)
	detected := http.DetectContentType(head[:n])
	okImage := isImage && strings.HasPrefix(detected, "image/")
	// Browsers/Go sniff most video containers as video/* or (for some mp4/mov)
	// as application/octet-stream; accept either when the extension is a known
	// video type so legitimate clips aren't wrongly rejected.
	okVideo := isVideo && (strings.HasPrefix(detected, "video/") || detected == "application/octet-stream")
	if !okImage && !okVideo {
		return httpx.BadRequest("unsupported_type", "The uploaded file is not a valid image or video")
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return httpx.Internal("failed to read upload")
	}

	// Namespace keys by account type + date for tidy storage. The key is fully
	// server-generated (uuid + validated ext) so a malicious filename can never
	// influence the path (no path traversal on the write side).
	p := auth.From(c)
	folder := "misc"
	if p != nil && (p.AccountType == "seller" || p.AccountType == "admin" || p.AccountType == "customer") {
		folder = p.AccountType
	}
	key := fmt.Sprintf("%s/%s/%s%s", folder, time.Now().Format("2006/01"), uuid.NewString(), ext)

	kind := storage.KindImage
	if isVideo {
		kind = storage.KindVideo
	}

	obj, err := h.provider.Upload(c.Context(), storage.UploadInput{
		Key:         key,
		ContentType: detected, // trust the sniffed type, not the client header
		Body:        f,
		Size:        fileHeader.Size,
		Kind:        kind,
	})
	if err != nil {
		return httpx.Internal("upload failed: " + err.Error())
	}
	return httpx.Created(c, fiber.Map{"key": obj.Key, "url": obj.URL, "size": obj.Size})
}
