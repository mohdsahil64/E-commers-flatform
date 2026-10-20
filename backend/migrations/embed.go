// Package migrations embeds the SQL migration files so they ship inside the
// compiled binary and can be run by the api / migrate commands.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS

// Dir is the directory (within the embedded FS) that holds the migrations.
const Dir = "."
