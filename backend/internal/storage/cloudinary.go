package storage

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/carlitos/backend/internal/config"
)

// cloudinaryProvider stores objects in Cloudinary and delivers them from
// Cloudinary's global CDN. It talks to the REST upload API directly over the
// standard library (a signed multipart POST) so no third-party SDK is needed.
//
// Key ⇄ public_id mapping
// -----------------------
// The storage key used across the app (e.g. "admin/2026/01/<uuid>.jpg") is
// reused verbatim as the Cloudinary public_id, including its extension. That
// keeps delivery URLs identical in shape to the local ones:
//
//	<publicBaseURL>/<key>  →  https://res.cloudinary.com/<cloud>/image/upload/<key>
//
// so the frontends' resolveMediaUrl(base, key) needs no change, and existing DB
// keys keep resolving once the one-time media migration has pushed the files.
type cloudinaryProvider struct {
	cloudName     string
	apiKey        string
	apiSecret     string
	folder        string
	publicBaseURL string
	http          *http.Client
}

func newCloudinaryProvider(cfg config.StorageConfig) (Provider, error) {
	if cfg.CloudinaryCloudName == "" || cfg.CloudinaryAPIKey == "" || cfg.CloudinaryAPISecret == "" {
		return nil, fmt.Errorf("cloudinary storage requires cloud name, api key and api secret")
	}
	base := strings.TrimRight(cfg.PublicBaseURL, "/")
	if base == "" {
		base = "https://res.cloudinary.com/" + cfg.CloudinaryCloudName + "/image/upload"
	}
	return &cloudinaryProvider{
		cloudName:     cfg.CloudinaryCloudName,
		apiKey:        cfg.CloudinaryAPIKey,
		apiSecret:     cfg.CloudinaryAPISecret,
		folder:        strings.Trim(cfg.CloudinaryFolder, "/"),
		publicBaseURL: base,
		http:          &http.Client{Timeout: 60 * time.Second},
	}, nil
}

func (p *cloudinaryProvider) Name() string { return "cloudinary" }

// Root is not meaningful for a remote provider; returns empty so the /media
// static route is skipped when Cloudinary is active.
func (p *cloudinaryProvider) Root() string { return "" }

// publicID derives the Cloudinary public_id from a storage key. Cloudinary
// treats the trailing extension as the resource format, so the public_id is the
// key without its extension (and with the optional folder prefix applied).
func (p *cloudinaryProvider) publicID(key string) string {
	k := strings.TrimLeft(key, "/")
	k = strings.TrimSuffix(k, pathExt(k))
	if p.folder != "" {
		return p.folder + "/" + k
	}
	return k
}

// resourceType maps a MediaKind to Cloudinary's resource type path segment.
func resourceType(kind MediaKind) string {
	if kind == KindVideo {
		return "video"
	}
	return "image"
}

// extIsVideo reports whether a storage key's extension is a known video type.
// Used so PublicURL (which only receives a key) can build a video delivery URL
// for keys that were stored as videos.
func extIsVideo(key string) bool {
	switch strings.ToLower(pathExt(key)) {
	case ".mp4", ".webm", ".mov", ".m4v":
		return true
	}
	return false
}

// deliveryBaseFor returns the CDN delivery prefix for the given resource type.
// publicBaseURL defaults to the image delivery prefix (.../image/upload); for a
// video key we swap the trailing "/image/upload" to "/video/upload".
func (p *cloudinaryProvider) deliveryBaseFor(video bool) string {
	if !video {
		return p.publicBaseURL
	}
	return strings.Replace(p.publicBaseURL, "/image/upload", "/video/upload", 1)
}

// PublicURL builds the CDN delivery URL for a stored key. It mirrors the local
// provider's shape (base + "/" + key) so callers/frontends are unchanged. The
// resource type (image vs video) is inferred from the key's extension.
func (p *cloudinaryProvider) PublicURL(key string) string {
	k := strings.TrimLeft(key, "/")
	if p.folder != "" {
		k = p.folder + "/" + k
	}
	return p.deliveryBaseFor(extIsVideo(key)) + "/" + k
}

// Upload pushes bytes to Cloudinary with a signed request, pinning the
// public_id to the storage key so the object is addressable by the same key the
// app already stores in the database. Existing objects with the same id are
// overwritten (idempotent re-runs during migration).
func (p *cloudinaryProvider) Upload(ctx context.Context, in UploadInput) (*Object, error) {
	publicID := p.publicID(in.Key)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)

	// Parameters that participate in the signature (alphabetical, per Cloudinary
	// signing rules): overwrite, public_id, timestamp.
	signParams := map[string]string{
		"overwrite": "true",
		"public_id": publicID,
		"timestamp": timestamp,
	}
	signature := p.sign(signParams)

	// Build multipart body: the signed params + api_key + the file bytes.
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	_ = w.WriteField("public_id", publicID)
	_ = w.WriteField("timestamp", timestamp)
	_ = w.WriteField("overwrite", "true")
	_ = w.WriteField("api_key", p.apiKey)
	_ = w.WriteField("signature", signature)

	part, err := w.CreateFormFile("file", pathBase(in.Key))
	if err != nil {
		return nil, fmt.Errorf("cloudinary: form file: %w", err)
	}
	written, err := io.Copy(part, in.Body)
	if err != nil {
		return nil, fmt.Errorf("cloudinary: read body: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("cloudinary: close writer: %w", err)
	}

	rtype := resourceType(in.Kind)
	endpoint := "https://api.cloudinary.com/v1_1/" + p.cloudName + "/" + rtype + "/upload"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &body)
	if err != nil {
		return nil, fmt.Errorf("cloudinary: new request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := p.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cloudinary: upload request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("cloudinary: upload failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(respBytes)))
	}

	var parsed struct {
		SecureURL string `json:"secure_url"`
		URL       string `json:"url"`
		Bytes     int64  `json:"bytes"`
		PublicID  string `json:"public_id"`
		Format    string `json:"format"`
		Error     *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return nil, fmt.Errorf("cloudinary: decode response: %w", err)
	}
	if parsed.Error != nil {
		return nil, fmt.Errorf("cloudinary: %s", parsed.Error.Message)
	}

	size := parsed.Bytes
	if size == 0 {
		size = written
	}
	// Serve via our own stable PublicURL shape (base + key) so it stays
	// consistent with how keys are stored and resolved everywhere.
	return &Object{
		Key:         in.Key,
		Size:        size,
		ContentType: in.ContentType,
		URL:         p.PublicURL(in.Key),
	}, nil
}

// Delete removes an object from Cloudinary (signed destroy call).
func (p *cloudinaryProvider) Delete(ctx context.Context, key string) error {
	publicID := p.publicID(key)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	signParams := map[string]string{
		"public_id": publicID,
		"timestamp": timestamp,
	}
	signature := p.sign(signParams)

	form := url.Values{}
	form.Set("public_id", publicID)
	form.Set("timestamp", timestamp)
	form.Set("api_key", p.apiKey)
	form.Set("signature", signature)

	rtype := "image"
	if extIsVideo(key) {
		rtype = "video"
	}
	endpoint := "https://api.cloudinary.com/v1_1/" + p.cloudName + "/" + rtype + "/destroy"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("cloudinary: new destroy request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.http.Do(req)
	if err != nil {
		return fmt.Errorf("cloudinary: destroy request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("cloudinary: destroy failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

// sign produces the Cloudinary request signature: SHA-1 of the signable params
// joined as "k=v&k=v" in alphabetical key order, with the api secret appended.
func (p *cloudinaryProvider) sign(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+params[k])
	}
	toSign := strings.Join(parts, "&") + p.apiSecret
	sum := sha1.Sum([]byte(toSign))
	return fmt.Sprintf("%x", sum[:])
}

// ---- tiny path helpers (avoid importing path/filepath for forward-slash keys) ----

func pathExt(key string) string {
	for i := len(key) - 1; i >= 0 && key[i] != '/'; i-- {
		if key[i] == '.' {
			return key[i:]
		}
	}
	return ""
}

func pathBase(key string) string {
	if i := strings.LastIndex(key, "/"); i >= 0 {
		return key[i+1:]
	}
	return key
}
