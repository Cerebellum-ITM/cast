package db

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// Sequence is the definition of a reusable ordered list of commands.
// For auto-saved chains, Name is a synthetic fingerprint "auto:<sha1>" so
// identical sequences dedup into a single row across executions.
type Sequence struct {
	ID          int64
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// SequenceStep is one ordered step inside a sequence.
type SequenceStep struct {
	ID              int64
	SequenceID      int64
	Position        int
	Command         string
	ContinueOnError bool
}

// SequenceRun is a single execution of a sequence.
type SequenceRun struct {
	ID         int64
	SequenceID int64
	Name       string
	Status     RunStatus
	StartedAt  time.Time
	FinishedAt sql.NullTime
	Duration   time.Duration
}

// SequenceSummary joins a Sequence with its step list and last-run stats, for
// the Chains history view.
type SequenceSummary struct {
	Sequence   Sequence
	Commands   []string
	RunCount   int
	LastRunAt  sql.NullTime
	LastStatus sql.NullInt64
}

// parseSQLiteTime parses the TEXT form SQLite returns for DATETIME aggregates
// (MIN/MAX/direct). Returns zero time for empty strings.
func parseSQLiteTime(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	// Try formats SQLite emits for datetime values.
	layouts := []string{
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02 15:04:05",
		time.RFC3339Nano,
		time.RFC3339,
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// ChainFingerprint returns the stable "auto:<sha1>" sequence name used to
// deduplicate auto-saved chains across executions.
func ChainFingerprint(commands []string) string {
	h := sha1.Sum([]byte(strings.Join(commands, "\x00")))
	return "auto:" + hex.EncodeToString(h[:8])
}

// UpsertChainSequence returns the sequence row for the given command list,
// creating (with steps) or touching updated_at as needed.
func (d *DB) UpsertChainSequence(ctx context.Context, commands []string) (int64, error) {
	if len(commands) == 0 {
		return 0, fmt.Errorf("chain sequence: at least one command required")
	}
	name := ChainFingerprint(commands)

	var id int64
	row := d.sql.QueryRowContext(ctx, `SELECT id FROM sequences WHERE name = ?`, name)
	err := row.Scan(&id)
	switch {
	case err == sql.ErrNoRows:
		res, err := d.sql.ExecContext(ctx, `
			INSERT INTO sequences (name, description) VALUES (?, ?)`,
			name, strings.Join(commands, " › "))
		if err != nil {
			return 0, fmt.Errorf("insert sequence: %w", err)
		}
		id, err = res.LastInsertId()
		if err != nil {
			return 0, err
		}
		for i, c := range commands {
			if _, err := d.sql.ExecContext(ctx, `
				INSERT INTO sequence_steps (sequence_id, position, command)
				VALUES (?, ?, ?)`, id, i+1, c); err != nil {
				return 0, fmt.Errorf("insert sequence_step: %w", err)
			}
		}
	case err != nil:
		return 0, fmt.Errorf("lookup sequence: %w", err)
	default:
		if _, err := d.sql.ExecContext(ctx,
			`UPDATE sequences SET updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id); err != nil {
			return 0, fmt.Errorf("touch sequence: %w", err)
		}
	}
	return id, nil
}

// StartSequenceRun inserts a sequence_runs row with status=running.
func (d *DB) StartSequenceRun(ctx context.Context, sequenceID int64, startedAt time.Time) (int64, error) {
	res, err := d.sql.ExecContext(ctx, `
		INSERT INTO sequence_runs (sequence_id, name, status, started_at)
		VALUES (?, ?, ?, ?)`,
		sequenceID, "", int(StatusRunning), startedAt.UTC())
	if err != nil {
		return 0, fmt.Errorf("insert sequence_run: %w", err)
	}
	return res.LastInsertId()
}

// FinishSequenceRun closes a sequence_runs row with a final status.
func (d *DB) FinishSequenceRun(ctx context.Context, id int64, status RunStatus, finishedAt time.Time, dur time.Duration) error {
	_, err := d.sql.ExecContext(ctx, `
		UPDATE sequence_runs
		SET status = ?, finished_at = ?, duration_ms = ?
		WHERE id = ?`,
		int(status), finishedAt.UTC(), dur.Milliseconds(), id)
	return err
}

// ListChainSummaries returns auto-saved chains (name prefix "auto:") joined
// with their steps and aggregated run stats, most-recently-used first.
func (d *DB) ListChainSummaries(ctx context.Context, limit int) ([]SequenceSummary, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := d.sql.QueryContext(ctx, `
		SELECT s.id, s.name, s.description, s.created_at, s.updated_at,
		       COUNT(sr.id)            AS run_count,
		       MAX(sr.started_at)      AS last_run_at,
		       (SELECT status FROM sequence_runs
		        WHERE sequence_id = s.id
		        ORDER BY id DESC LIMIT 1) AS last_status
		FROM sequences s
		LEFT JOIN sequence_runs sr ON sr.sequence_id = s.id
		WHERE s.name LIKE 'auto:%'
		GROUP BY s.id
		ORDER BY COALESCE(last_run_at, s.updated_at) DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SequenceSummary
	for rows.Next() {
		var s SequenceSummary
		var (
			runCount     sql.NullInt64
			lastRunAtStr sql.NullString
			createdStr   sql.NullString
			updatedStr   sql.NullString
		)
		if err := rows.Scan(
			&s.Sequence.ID, &s.Sequence.Name, &s.Sequence.Description,
			&createdStr, &updatedStr,
			&runCount, &lastRunAtStr, &s.LastStatus,
		); err != nil {
			return nil, err
		}
		if runCount.Valid {
			s.RunCount = int(runCount.Int64)
		}
		if createdStr.Valid {
			if t, ok := parseSQLiteTime(createdStr.String); ok {
				s.Sequence.CreatedAt = t
			}
		}
		if updatedStr.Valid {
			if t, ok := parseSQLiteTime(updatedStr.String); ok {
				s.Sequence.UpdatedAt = t
			}
		}
		if lastRunAtStr.Valid {
			if t, ok := parseSQLiteTime(lastRunAtStr.String); ok {
				s.LastRunAt = sql.NullTime{Time: t, Valid: true}
			}
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range out {
		cmds, err := d.sequenceCommands(ctx, out[i].Sequence.ID)
		if err != nil {
			return nil, err
		}
		out[i].Commands = cmds
	}
	return out, nil
}

// ChainRunRecord is a single chain execution for the history tab.
type ChainRunRecord struct {
	ID         int64
	SequenceID int64
	Status     RunStatus
	StartedAt  time.Time
	Duration   time.Duration
	Commands   []string
}

// ListChainRuns returns the most recent auto-saved chain executions joined
// with their step list, newest first.
func (d *DB) ListChainRuns(ctx context.Context, limit int) ([]ChainRunRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := d.sql.QueryContext(ctx, `
		SELECT sr.id, sr.sequence_id, sr.status, sr.started_at, sr.duration_ms
		FROM sequence_runs sr
		JOIN sequences s ON s.id = sr.sequence_id
		WHERE s.name LIKE 'auto:%'
		ORDER BY sr.id DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ChainRunRecord
	for rows.Next() {
		var (
			r         ChainRunRecord
			startedAt sql.NullString
			durMs     sql.NullInt64
		)
		if err := rows.Scan(&r.ID, &r.SequenceID, &r.Status, &startedAt, &durMs); err != nil {
			return nil, err
		}
		if startedAt.Valid {
			if t, ok := parseSQLiteTime(startedAt.String); ok {
				r.StartedAt = t
			}
		}
		if durMs.Valid {
			r.Duration = time.Duration(durMs.Int64) * time.Millisecond
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		cmds, err := d.sequenceCommands(ctx, out[i].SequenceID)
		if err != nil {
			return nil, err
		}
		out[i].Commands = cmds
	}
	return out, nil
}

// PruneChainRuns keeps the `keep` most recent sequence_runs, deleting older
// rows along with any sequence rows that become orphaned.
func (d *DB) PruneChainRuns(ctx context.Context, keep int) error {
	if keep <= 0 {
		return nil
	}
	if _, err := d.sql.ExecContext(ctx, `
		DELETE FROM sequence_runs
		WHERE id NOT IN (SELECT id FROM sequence_runs ORDER BY id DESC LIMIT ?)`, keep); err != nil {
		return err
	}
	// Remove auto-saved sequences no longer referenced by any run.
	_, err := d.sql.ExecContext(ctx, `
		DELETE FROM sequences
		WHERE name LIKE 'auto:%'
		  AND id NOT IN (SELECT DISTINCT sequence_id FROM sequence_runs WHERE sequence_id IS NOT NULL)`)
	return err
}

func (d *DB) sequenceCommands(ctx context.Context, sequenceID int64) ([]string, error) {
	rows, err := d.sql.QueryContext(ctx, `
		SELECT command FROM sequence_steps
		WHERE sequence_id = ? ORDER BY position ASC`, sequenceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
