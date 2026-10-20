package database_test

import (
	"strings"
	"testing"

	"github.com/carlitos/backend/internal/database"
	"github.com/carlitos/backend/migrations"
)

// TestMigrationsLoad verifies that every embedded migration is discovered,
// ordered by version, and has non-empty up/down SQL. This catches naming or
// embed mistakes without needing a live database.
func TestMigrationsLoad(t *testing.T) {
	migs, err := database.LoadMigrations(migrations.FS, migrations.Dir)
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	if len(migs) < 15 {
		t.Fatalf("expected at least 15 migrations, got %d", len(migs))
	}

	prev := ""
	for _, m := range migs {
		if m.Version <= prev {
			t.Errorf("migrations out of order: %q after %q", m.Version, prev)
		}
		prev = m.Version
		if strings.TrimSpace(m.UpSQL) == "" {
			t.Errorf("migration %s_%s has empty up SQL", m.Version, m.Name)
		}
		if strings.TrimSpace(m.DownSQL) == "" {
			t.Errorf("migration %s_%s has empty down SQL", m.Version, m.Name)
		}
	}
}
