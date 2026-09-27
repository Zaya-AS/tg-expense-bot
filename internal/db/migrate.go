package db

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var upMigrationName = regexp.MustCompile(`^([0-9]+)_.+\.up\.sql$`)

type migration struct {
	version int64
	name    string
	sql     string
}

func Migrate(ctx context.Context, pool *pgxpool.Pool, directory string) error {
	files, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}

	var migrations []migration
	versions := make(map[int64]bool)
	for _, file := range files {
		match := upMigrationName.FindStringSubmatch(file.Name())
		if file.IsDir() || match == nil {
			continue
		}
		version, err := strconv.ParseInt(match[1], 10, 64)
		if err != nil || version < 1 || versions[version] {
			return fmt.Errorf("invalid or duplicate migration version in %s", file.Name())
		}
		data, err := os.ReadFile(filepath.Join(directory, file.Name()))
		if err != nil {
			return fmt.Errorf("read migration %s: %w", file.Name(), err)
		}
		versions[version] = true
		migrations = append(migrations, migration{version: version, name: file.Name(), sql: string(data)})
	}
	if len(migrations) == 0 || !versions[1] {
		return fmt.Errorf("initial migration is missing from %s", directory)
	}
	sort.Slice(migrations, func(i, j int) bool { return migrations[i].version < migrations[j].version })

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin migrations: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(812834991)`); err != nil {
		return fmt.Errorf("lock migrations: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version BIGINT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	rows, err := tx.Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return fmt.Errorf("read applied migrations: %w", err)
	}
	applied := make(map[int64]bool)
	for rows.Next() {
		var version int64
		if err := rows.Scan(&version); err != nil {
			rows.Close()
			return fmt.Errorf("scan applied migration: %w", err)
		}
		applied[version] = true
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("read applied migrations: %w", err)
	}
	rows.Close()
	for version := range applied {
		if !versions[version] {
			return fmt.Errorf("database has migration version %d that is missing from %s", version, directory)
		}
	}

	// The first release created these tables through PostgreSQL's init directory.
	// Record that migration on existing installations without touching their data.
	if !applied[1] {
		var users, categories, expenses bool
		err := tx.QueryRow(ctx, `
			SELECT to_regclass('users') IS NOT NULL,
				to_regclass('categories') IS NOT NULL,
				to_regclass('expenses') IS NOT NULL
		`).Scan(&users, &categories, &expenses)
		if err != nil {
			return fmt.Errorf("inspect existing schema: %w", err)
		}
		if users && categories && expenses {
			if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES (1)`); err != nil {
				return fmt.Errorf("record existing schema: %w", err)
			}
			applied[1] = true
		} else if users || categories || expenses {
			return fmt.Errorf("existing core schema is incomplete; migrations stopped")
		}
	}

	for _, m := range migrations {
		if applied[m.version] {
			continue
		}
		if _, err := tx.Exec(ctx, m.sql, pgx.QueryExecModeSimpleProtocol); err != nil {
			return fmt.Errorf("apply migration %s: %w", m.name, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, m.version); err != nil {
			return fmt.Errorf("record migration %s: %w", m.name, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit migrations: %w", err)
	}
	return nil
}
