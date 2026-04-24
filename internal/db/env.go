package db

import (
	"context"
	"database/sql"
	"time"
)

// EnvChange records one mutation to an environment variable.
type EnvChange struct {
	ID        int64
	Key       string
	OldValue  sql.NullString // NULL means the var was newly created
	NewValue  string
	Sensitive bool
	EnvFile   string
	ChangedAt time.Time
	ChangedBy string // "user" | "cli"
}

// TimeStr returns ChangedAt as HH:MM for today, or "Jan 02" for older entries.
func (e EnvChange) TimeStr() string {
	if e.ChangedAt.IsZero() {
		return ""
	}
	t := e.ChangedAt.Local()
	if t.Year() == time.Now().Year() &&
		t.YearDay() == time.Now().YearDay() {
		return t.Format("15:04")
	}
	return t.Format("Jan 02")
}

// InsertEnvChange persists one env var change and returns the new row ID.
func (d *DB) InsertEnvChange(ctx context.Context, c EnvChange) (int64, error) {
	res, err := d.sql.ExecContext(ctx, `
		INSERT INTO env_changes (key, old_value, new_value, sensitive, env_file, changed_at, changed_by)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		c.Key,
		c.OldValue,
		c.NewValue,
		boolToInt(c.Sensitive),
		nonEmpty(c.EnvFile, ".env"),
		c.ChangedAt.UTC(),
		nonEmpty(c.ChangedBy, "user"),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// RecentEnvChanges returns the most recent changes across all keys, newest first.
func (d *DB) RecentEnvChanges(ctx context.Context, limit int) ([]EnvChange, error) {
	if limit <= 0 {
		return nil, nil
	}
	rows, err := d.sql.QueryContext(ctx, `
		SELECT id, key, old_value, new_value, sensitive, env_file, changed_at, changed_by
		FROM env_changes
		ORDER BY id DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEnvChanges(rows)
}

// EnvKeyHistory returns change history for a single key, newest first.
func (d *DB) EnvKeyHistory(ctx context.Context, key string, limit int) ([]EnvChange, error) {
	if limit <= 0 {
		return nil, nil
	}
	rows, err := d.sql.QueryContext(ctx, `
		SELECT id, key, old_value, new_value, sensitive, env_file, changed_at, changed_by
		FROM env_changes
		WHERE key = ?
		ORDER BY id DESC
		LIMIT ?`, key, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEnvChanges(rows)
}

func scanEnvChanges(rows *sql.Rows) ([]EnvChange, error) {
	var out []EnvChange
	for rows.Next() {
		var (
			c         EnvChange
			sensitive int
		)
		if err := rows.Scan(
			&c.ID, &c.Key, &c.OldValue, &c.NewValue,
			&sensitive, &c.EnvFile, &c.ChangedAt, &c.ChangedBy,
		); err != nil {
			return nil, err
		}
		c.Sensitive = sensitive == 1
		out = append(out, c)
	}
	return out, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
