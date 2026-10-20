// Command migrate applies or rolls back SQL migrations.
//
// Usage:
//
//	go run ./cmd/migrate up      # apply all pending migrations
//	go run ./cmd/migrate down    # roll back the most recent migration
//	go run ./cmd/migrate status  # show applied vs pending
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/carlitos/backend/internal/app"
	"github.com/carlitos/backend/internal/database"
	"github.com/carlitos/backend/migrations"
)

func main() {
	cmd := "up"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	ctx := context.Background()
	a, err := app.NewWithoutRedis(ctx)
	if err != nil {
		fail(err)
	}
	defer a.Close()

	migs, err := database.LoadMigrations(migrations.FS, migrations.Dir)
	if err != nil {
		fail(err)
	}

	switch cmd {
	case "up":
		n, err := database.MigrateUp(ctx, a.DB.Pool, migs)
		if err != nil {
			fail(err)
		}
		fmt.Printf("✔ applied %d migration(s)\n", n)
	case "down":
		if err := database.MigrateDown(ctx, a.DB.Pool, migs); err != nil {
			fail(err)
		}
		fmt.Println("✔ rolled back last migration")
	case "status":
		fmt.Printf("discovered %d migration file group(s):\n", len(migs))
		for _, m := range migs {
			fmt.Printf("  - %s_%s\n", m.Version, m.Name)
		}
	default:
		fmt.Printf("unknown command %q (use: up | down | status)\n", cmd)
		os.Exit(2)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "migrate error:", err)
	os.Exit(1)
}
