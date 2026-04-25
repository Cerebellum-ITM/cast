package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// LastRun is the persisted "rerun" target for a project. Commands holds the
// ordered list of make targets dispatched on the most recent action: a
// single-element slice for a normal run, ≥2 for a chain. ExtraVars apply to
// single-command pick runs only (chains skip the picker).
type LastRun struct {
	Commands  []string
	ExtraVars []string
}

// IsChain reports whether this rerun encodes a chain (≥ 2 steps).
func (l LastRun) IsChain() bool { return len(l.Commands) > 1 }

// GetLastRun returns the saved rerun target for projectDir, or (nil, nil)
// when no row exists yet.
func (d *DB) GetLastRun(ctx context.Context, projectDir string) (*LastRun, error) {
	if projectDir == "" {
		return nil, nil
	}
	var cmdsJSON, extrasJSON string
	row := d.sql.QueryRowContext(ctx,
		`SELECT commands, extra_vars FROM project_last_runs WHERE project_dir = ?`,
		projectDir)
	if err := row.Scan(&cmdsJSON, &extrasJSON); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get last run: %w", err)
	}
	var cmds, extras []string
	if err := json.Unmarshal([]byte(cmdsJSON), &cmds); err != nil {
		return nil, fmt.Errorf("decode last_run commands: %w", err)
	}
	if extrasJSON != "" {
		if err := json.Unmarshal([]byte(extrasJSON), &extras); err != nil {
			return nil, fmt.Errorf("decode last_run extra_vars: %w", err)
		}
	}
	if len(cmds) == 0 {
		return nil, nil
	}
	return &LastRun{Commands: cmds, ExtraVars: extras}, nil
}

// UpsertLastRun writes (or replaces) the rerun target for projectDir. Empty
// projectDir or commands yields no-op so callers don't need to guard.
func (d *DB) UpsertLastRun(ctx context.Context, projectDir string, lr LastRun) error {
	if projectDir == "" || len(lr.Commands) == 0 {
		return nil
	}
	cmdsJSON, err := json.Marshal(lr.Commands)
	if err != nil {
		return err
	}
	extras := lr.ExtraVars
	if extras == nil {
		extras = []string{}
	}
	extrasJSON, err := json.Marshal(extras)
	if err != nil {
		return err
	}
	_, err = d.sql.ExecContext(ctx,
		`INSERT INTO project_last_runs(project_dir, commands, extra_vars, updated_at)
		 VALUES(?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(project_dir) DO UPDATE SET
		   commands   = excluded.commands,
		   extra_vars = excluded.extra_vars,
		   updated_at = CURRENT_TIMESTAMP`,
		projectDir, string(cmdsJSON), string(extrasJSON))
	if err != nil {
		return fmt.Errorf("upsert last run: %w", err)
	}
	return nil
}
