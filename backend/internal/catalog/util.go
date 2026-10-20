package catalog

import (
	"strings"

	"github.com/google/uuid"
)

// Slugify converts a title into a URL-safe slug.
func Slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash {
				b.WriteRune('-')
				prevDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "item"
	}
	return out
}

// shortID returns a short random suffix to keep slugs/SKUs unique.
func shortID() string {
	return strings.ReplaceAll(uuid.NewString(), "-", "")[:8]
}
