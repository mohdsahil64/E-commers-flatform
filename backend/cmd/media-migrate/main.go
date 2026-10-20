
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/carlitos/backend/internal/config"
	"github.com/carlitos/backend/internal/storage"
)

func main() {
	var (
		dryRun  bool
		rootDir string
	)
	flag.BoolVar(&dryRun, "dry-run", false, "list files that would be uploaded without uploading")
	flag.StringVar(&rootDir, "root", "", "local media root to migrate (defaults to STORAGE_LOCAL_ROOT)")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fail(err)
	}

	if rootDir == "" {
		rootDir = cfg.Storage.LocalRoot
	}
	if rootDir == "" {
		rootDir = "./.data/storage"
	}

	// Build a Cloudinary provider explicitly (regardless of the auto-selected
	// provider) so the migration always targets Cloudinary.
	cldCfg := cfg.Storage
	cldCfg.Provider = "cloudinary"
	provider, err := storage.New(cldCfg)
	if err != nil {
		fail(fmt.Errorf("cloudinary not configured: %w", err))
	}
	fmt.Printf("Target storage provider: %s\n", provider.Name())
	fmt.Printf("Local media root:        %s\n", rootDir)
	if dryRun {
		fmt.Println("Mode:                    DRY RUN (no uploads)")
	}

	info, err := os.Stat(rootDir)
	if err != nil || !info.IsDir() {
		fail(fmt.Errorf("local media root %q is not a directory (nothing to migrate): %v", rootDir, err))
	}

	allowedImages := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".avif": true, ".gif": true,
	}
	allowedVideos := map[string]bool{
		".mp4": true, ".webm": true, ".mov": true, ".m4v": true,
	}

	ctx := context.Background()
	var uploaded, skipped, failed int

	walkErr := filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		isImg := allowedImages[ext]
		isVid := allowedVideos[ext]
		if !isImg && !isVid {
			skipped++
			return nil
		}

		// Derive the storage key = path relative to root, with forward slashes.
		rel, relErr := filepath.Rel(rootDir, path)
		if relErr != nil {
			failed++
			fmt.Fprintf(os.Stderr, "  ! %s: %v\n", path, relErr)
			return nil
		}
		key := filepath.ToSlash(rel)

		if dryRun {
			fmt.Printf("  would upload: %s\n", key)
			uploaded++
			return nil
		}

		f, openErr := os.Open(path)
		if openErr != nil {
			failed++
			fmt.Fprintf(os.Stderr, "  ! %s: %v\n", key, openErr)
			return nil
		}
		fi, _ := f.Stat()
		var size int64
		if fi != nil {
			size = fi.Size()
		}
		kind := storage.KindImage
		if isVid {
			kind = storage.KindVideo
		}
		obj, upErr := provider.Upload(ctx, storage.UploadInput{
			Key:         key,
			ContentType: contentTypeFor(ext),
			Body:        f,
			Size:        size,
			Kind:        kind,
		})
		f.Close()
		if upErr != nil {
			failed++
			fmt.Fprintf(os.Stderr, "  ! %s: %v\n", key, upErr)
			return nil
		}
		uploaded++
		fmt.Printf("  ✔ %s → %s\n", key, obj.URL)
		return nil
	})
	if walkErr != nil {
		fail(walkErr)
	}

	fmt.Printf("\nDone. uploaded=%d skipped(non-image)=%d failed=%d\n", uploaded, skipped, failed)
	if failed > 0 {
		os.Exit(1)
	}
}

func contentTypeFor(ext string) string {
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".avif":
		return "image/avif"
	case ".gif":
		return "image/gif"
	case ".mp4", ".m4v":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".mov":
		return "video/quicktime"
	default:
		return "application/octet-stream"
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "media-migrate error:", err)
	os.Exit(1)
}
