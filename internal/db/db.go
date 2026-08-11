package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	_ "github.com/tursodatabase/libsql-client-go/libsql"
	_ "modernc.org/sqlite"
)

// The line below is a compiler directive, NOT a comment — it is what pulls
// the .sql files into the binary. Deleting it still compiles: migrationsFS
// just becomes an empty filesystem, and the server dies at startup with
// "open migrations: file does not exist". Any tool that strips comments
// must be configured to leave //go: directives alone.
//
//go:embed migrations/*.sql
var migrationsFS embed.FS

func Open(ctx context.Context, target string) (*sql.DB, error) {
	if isRemoteURL(target) {
		return openRemote(ctx, target)
	}
	return openLocalFile(ctx, target)
}

func isRemoteURL(target string) bool {
	return strings.HasPrefix(target, "libsql://") ||
		strings.HasPrefix(target, "http://") ||
		strings.HasPrefix(target, "https://")
}

func openRemote(ctx context.Context, url string) (*sql.DB, error) {
	sqlDB, err := sql.Open("libsql", url)
	if err != nil {
		return nil, fmt.Errorf("open libsql: %w", err)
	}
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(time.Hour)

	if err := sqlDB.PingContext(ctx); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("reach libsql database: %w", err)
	}
	if err := migrate(ctx, sqlDB); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return sqlDB, nil
}
func openLocalFile(ctx context.Context, path string) (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"file:%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)",
		path,
	)
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	sqlDB.SetMaxOpenConns(1)

	if err := migrate(ctx, sqlDB); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return sqlDB, nil
}

func migrate(ctx context.Context, sqlDB *sql.DB) error {
	if _, err := sqlDB.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename   TEXT PRIMARY KEY,
			applied_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`); err != nil {
		return err
	}

	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		var already int
		if err := sqlDB.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM schema_migrations WHERE filename = ?`, name,
		).Scan(&already); err != nil {
			return err
		}
		if already > 0 {
			continue
		}

		contents, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}

		tx, err := sqlDB.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, string(contents)); err != nil {
			tx.Rollback()
			return fmt.Errorf("apply %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations (filename) VALUES (?)`, name,
		); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
