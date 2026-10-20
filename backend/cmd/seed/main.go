// Command seed loads development seed data (admin, seller, customer, catalog,
// sample products, orders, banners, collections). The full seed is implemented
// alongside the domain modules.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/carlitos/backend/internal/app"
	"github.com/carlitos/backend/internal/seed"
)

func main() {
	ctx := context.Background()
	a, err := app.New(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "seed error:", err)
		os.Exit(1)
	}
	defer a.Close()

	if err := seed.Run(ctx, a); err != nil {
		fmt.Fprintln(os.Stderr, "seed error:", err)
		os.Exit(1)
	}
	fmt.Println("✔ seed complete")
}
