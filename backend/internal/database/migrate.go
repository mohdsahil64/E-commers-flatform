package database

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Migration represents a single ordered SQL migration file.
type Migration struct {
	Version string // e.g. "0001"
	Name    string // e.g. "init"
	UpSQL   string
	DownSQL string
}

// migrationsTableDDL tracks which migrations have been applied.
const migrationsTableDDL = `
CREATE TABLE IF NOT EXISTS schema_migrations (
	version     TEXT PRIMARY KEY,
	name        TEXT NOT NULL,
	applied_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);`

// LoadMigrations reads *.up.sql / *.down.sql pairs from an fs.FS. Files must be
// named like `0001_init.up.sql` and `0001_init.down.sql`.
func LoadMigrations(fsys fs.FS, dir string) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}

	ups := map[string]Migration{}
	order := []string{}

	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			continue
		}
		isUp := strings.HasSuffix(name, ".up.sql")
		isDown := strings.HasSuffix(name, ".down.sql")
		if !isUp && !isDown {
			continue
		}
		base := strings.TrimSuffix(strings.TrimSuffix(name, ".up.sql"), ".down.sql")
		parts := strings.SplitN(base, "_", 2)
		version := parts[0]
		mName := version
		if len(parts) == 2 {
			mName = parts[1]
		}

		content, err := fs.ReadFile(fsys, path.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", name, err)
		}

		m := ups[version]
		m.Version = version
		m.Name = mName
		if isUp {
			m.UpSQL = string(content)
			order = append(order, version)
		} else {
			m.DownSQL = string(content)
		}
		ups[version] = m
	}

	sort.Strings(order)
	out := make([]Migration, 0, len(order))
	seen := map[string]bool{}
	for _, v := range order {
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, ups[v])
	}
	return out, nil
}

// MigrateUp applies all migrations that have not yet been recorded.
func MigrateUp(ctx context.Context, pool *pgxpool.Pool, migrations []Migration) (int, error) {
	if _, err := pool.Exec(ctx, migrationsTableDDL); err != nil {
		return 0, fmt.Errorf("ensure migrations table: %w", err)
	}

	applied, err := appliedVersions(ctx, pool)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, m := range migrations {
		if applied[m.Version] {
			continue
		}
		if strings.TrimSpace(m.UpSQL) == "" {
			continue
		}
		err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, m.UpSQL); err != nil {
				return fmt.Errorf("apply %s_%s: %w", m.Version, m.Name, err)
			}
			_, err := tx.Exec(ctx,
				`INSERT INTO schema_migrations (version, name) VALUES ($1, $2)`,
				m.Version, m.Name)
			return err
		})
		if err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

// MigrateDown rolls back the most recently applied migration.
func MigrateDown(ctx context.Context, pool *pgxpool.Pool, migrations []Migration) error {
	applied, err := appliedVersions(ctx, pool)
	if err != nil {
		return err
	}
	// Find highest applied version present in the set.
	byVersion := map[string]Migration{}
	versions := []string{}
	for _, m := range migrations {
		byVersion[m.Version] = m
		if applied[m.Version] {
			versions = append(versions, m.Version)
		}
	}
	if len(versions) == 0 {
		return nil
	}
	sort.Strings(versions)
	last := versions[len(versions)-1]
	m := byVersion[last]

	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if strings.TrimSpace(m.DownSQL) != "" {
			if _, err := tx.Exec(ctx, m.DownSQL); err != nil {
				return fmt.Errorf("rollback %s: %w", last, err)
			}
		}
		_, err := tx.Exec(ctx, `DELETE FROM schema_migrations WHERE version = $1`, last)
		return err
	})
}

func appliedVersions(ctx context.Context, pool *pgxpool.Pool) (map[string]bool, error) {
	rows, err := pool.Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		// Table may not exist yet.
		return map[string]bool{}, nil
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out[v] = true
	}
	return out, rows.Err()
}
