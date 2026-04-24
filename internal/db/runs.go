package db

import (
	"context"
	"database/sql"
	"time"
)

// Run is a single command execution record.
type Run struct {
	ID            int64
	Command       string
	Source        string
	Status        RunStatus
	ExitCode      sql.NullInt64
	StartedAt     time.Time
	FinishedAt    sql.NullTime
	Duration      time.Duration
	Env           string
	SequenceRunID sql.NullInt64
	StepIndex     sql.NullInt64
}

// DurationStr renders the run duration the same way the TUI used to display it
// (sub-second precision → ms, otherwise truncated to seconds).
func (r Run) DurationStr() string { return formatDuration(r.Duration) }

// TimeStr renders the start time as HH:MM:SS to match the legacy display.
func (r Run) TimeStr() string {
	if r.StartedAt.IsZero() {
		return ""
	}
	return r.StartedAt.Local().Format("15:04:05")
}

// NewRun builds a Run from a command execution result.
// interrupted=true maps to StatusInterrupted regardless of err (SIGINT typically
// surfaces as a non-nil error from cmd.Wait()).
func NewRun(command string, env string, startedAt time.Time, dur time.Duration, err error, interrupted bool) Run {
	status := StatusSuccess
	switch {
	case interrupted:
		status = StatusInterrupted
	case err != nil:
		status = StatusError
	}
	finished := startedAt.Add(dur)
	return Run{
		Command:    command,
		Source:     "makefile",
		Status:     status,
		StartedAt:  startedAt,
		FinishedAt: sql.NullTime{Time: finished, Valid: true},
		Duration:   dur,
		Env:        env,
	}
}

// InsertRun persists a Run and returns its assigned ID.
func (d *DB) InsertRun(ctx context.Context, r Run) (int64, error) {
	res, err := d.sql.ExecContext(ctx, `
		INSERT INTO runs
			(command, source, status, exit_code, started_at, finished_at,
			 duration_ms, env, sequence_run_id, step_index)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.Command,
		nonEmpty(r.Source, "makefile"),
		int(r.Status),
		r.ExitCode,
		r.StartedAt.UTC(),
		nullableUTCTime(r.FinishedAt),
		r.Duration.Milliseconds(),
		r.Env,
		r.SequenceRunID,
		r.StepIndex,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// RecentRuns returns the most recent runs, newest first, up to limit rows.
// A non-positive limit returns nil, nil.
func (d *DB) RecentRuns(ctx context.Context, limit int) ([]Run, error) {
	if limit <= 0 {
		return nil, nil
	}
	rows, err := d.sql.QueryContext(ctx, `
		SELECT id, command, source, status, exit_code, started_at, finished_at,
		       duration_ms, env, sequence_run_id, step_index
		FROM runs
		ORDER BY id DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Run
	for rows.Next() {
		var (
			r          Run
			durationMs sql.NullInt64
			env        sql.NullString
		)
		if err := rows.Scan(
			&r.ID, &r.Command, &r.Source, &r.Status, &r.ExitCode,
			&r.StartedAt, &r.FinishedAt, &durationMs, &env,
			&r.SequenceRunID, &r.StepIndex,
		); err != nil {
			return nil, err
		}
		if durationMs.Valid {
			r.Duration = time.Duration(durationMs.Int64) * time.Millisecond
		}
		if env.Valid {
			r.Env = env.String
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// PruneRuns keeps only the `keep` most recent rows in the runs table.
// Safe no-op for keep <= 0.
func (d *DB) PruneRuns(ctx context.Context, keep int) error {
	if keep <= 0 {
		return nil
	}
	_, err := d.sql.ExecContext(ctx, `
		DELETE FROM runs
		WHERE id NOT IN (
			SELECT id FROM runs ORDER BY id DESC LIMIT ?
		)`, keep)
	return err
}

func nonEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func nullableUTCTime(n sql.NullTime) sql.NullTime {
	if !n.Valid {
		return n
	}
	return sql.NullTime{Time: n.Time.UTC(), Valid: true}
}

func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}
	if d < time.Second {
		return d.Truncate(time.Millisecond).String()
	}
	return d.Truncate(time.Second).String()
}
