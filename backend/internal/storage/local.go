package storage

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/carlitos/backend/internal/config"
)

// localProvider stores objects on the local filesystem. Files are written under
// root and served to clients via the API's /media static route (see router).
type localProvider struct {
	root          string
	publicBaseURL string
}

func newLocalProvider(cfg config.StorageConfig) (Provider, error) {
	root := cfg.LocalRoot
	if root == "" {
		root = "./.data/storage"
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	return &localProvider{
		root:          root,
		publicBaseURL: strings.TrimRight(cfg.PublicBaseURL, "/"),
	}, nil
}

// Root is the on-disk directory where media is stored (used by the /media
// static route to serve files).
func (p *localProvider) Root() string { return p.root }

func (p *localProvider) Upload(ctx context.Context, in UploadInput) (*Object, error) {
	dest := filepath.Join(p.root, filepath.FromSlash(in.Key))
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return nil, err
	}
	f, err := os.Create(dest)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	n, err := io.Copy(f, in.Body)
	if err != nil {
		return nil, err
	}
	return &Object{Key: in.Key, Size: n, ContentType: in.ContentType, URL: p.PublicURL(in.Key)}, nil
}

func (p *localProvider) Delete(ctx context.Context, key string) error {
	return os.Remove(filepath.Join(p.root, filepath.FromSlash(key)))
}

func (p *localProvider) PublicURL(key string) string {
	if p.publicBaseURL != "" {
		return p.publicBaseURL + "/" + key
	}
	return "/media/" + key
}

func (p *localProvider) Name() string { return "local" }
