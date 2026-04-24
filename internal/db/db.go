// Package db provides SQLite-backed persistence for cast.
//
// The database lives at ~/.config/cast/cast.db (configurable) and is created
// automatically on first open. All schema management is handled via embedded
// forward-only migrations under internal/db/migrations/.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// DB wraps *sql.DB with cast-specific helpers.
type DB struct {
	sql *sql.DB
}

// Open opens (or creates) the SQLite database at path and applies pending
// migrations. The parent directory is created if missing.
func Open(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("creating db dir: %w", err)
	}

	dsn := dsnFor(path)
	sdb, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite: %w", err)
	}
	// SQLite + single process → one writer is the safe default.
	sdb.SetMaxOpenConns(1)

	if err := sdb.PingContext(context.Background()); err != nil {
		_ = sdb.Close()
		return nil, fmt.Errorf("pinging sqlite: %w", err)
	}

	d := &DB{sql: sdb}
	if err := d.migrate(context.Background()); err != nil {
		_ = sdb.Close()
		return nil, err
	}
	return d, nil
}

// Close releases the underlying sql.DB.
func (d *DB) Close() error {
	if d == nil || d.sql == nil {
		return nil
	}
	return d.sql.Close()
}

// SQL exposes the underlying *sql.DB for advanced callers (tests, future use).
func (d *DB) SQL() *sql.DB { return d.sql }

// dsnFor builds a modernc.org/sqlite DSN that enables FK, WAL, and a
// reasonable busy timeout. In-memory paths pass through unchanged.
func dsnFor(path string) string {
	if path == ":memory:" {
		return path
	}
	v := url.Values{}
	v.Add("_pragma", "foreign_keys(1)")
	v.Add("_pragma", "journal_mode(WAL)")
	v.Add("_pragma", "busy_timeout(5000)")
	return path + "?" + v.Encode()
}
