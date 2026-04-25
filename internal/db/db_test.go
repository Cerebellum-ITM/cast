package db

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func tempDB(t *testing.T) *DB {
	t.Helper()
	dir := t.TempDir()
	d, err := Open(filepath.Join(dir, "cast.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

func TestMigrateIdempotent(t *testing.T) {
	d := tempDB(t)
	// Running migrate again should be a no-op.
	if err := d.migrate(context.Background()); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	var n int
	if err := d.sql.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	wantMigrations := 3 // update when new migrations are added
	if n != wantMigrations {
		t.Fatalf("expected %d migration rows, got %d", wantMigrations, n)
	}
}

func TestInsertAndRecentRuns(t *testing.T) {
	d := tempDB(t)
	ctx := context.Background()

	started := time.Now().Add(-2 * time.Second)
	run := NewRun("build", "local", started, 1500*time.Millisecond, nil, false)
	id, err := d.InsertRun(ctx, run)
	if err != nil {
		t.Fatalf("InsertRun: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected positive id, got %d", id)
	}

	recent, err := d.RecentRuns(ctx, 10)
	if err != nil {
		t.Fatalf("RecentRuns: %v", err)
	}
	if len(recent) != 1 {
		t.Fatalf("expected 1 row, got %d", len(recent))
	}
	got := recent[0]
	if got.Command != "build" {
		t.Errorf("Command = %q, want build", got.Command)
	}
	if got.Status != StatusSuccess {
		t.Errorf("Status = %d, want StatusSuccess", got.Status)
	}
	if got.Duration != 1500*time.Millisecond {
		t.Errorf("Duration = %v, want 1.5s", got.Duration)
	}
	if got.Env != "local" {
		t.Errorf("Env = %q, want local", got.Env)
	}
}

func TestRecentRunsOrderingAndLimit(t *testing.T) {
	d := tempDB(t)
	ctx := context.Background()

	start := time.Now().Add(-10 * time.Second)
	for i, name := range []string{"a", "b", "c"} {
		r := NewRun(name, "local", start.Add(time.Duration(i)*time.Second), time.Second, nil, false)
		if _, err := d.InsertRun(ctx, r); err != nil {
			t.Fatalf("InsertRun %s: %v", name, err)
		}
	}
	recent, err := d.RecentRuns(ctx, 2)
	if err != nil {
		t.Fatalf("RecentRuns: %v", err)
	}
	if len(recent) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(recent))
	}
	if recent[0].Command != "c" || recent[1].Command != "b" {
		t.Errorf("wrong order: %q, %q", recent[0].Command, recent[1].Command)
	}
}

func TestPruneRuns(t *testing.T) {
	d := tempDB(t)
	ctx := context.Background()

	start := time.Now().Add(-time.Minute)
	for i := 0; i < 5; i++ {
		r := NewRun("cmd", "local", start.Add(time.Duration(i)*time.Second), time.Second, nil, false)
		if _, err := d.InsertRun(ctx, r); err != nil {
			t.Fatalf("InsertRun: %v", err)
		}
	}
	if err := d.PruneRuns(ctx, 3); err != nil {
		t.Fatalf("PruneRuns: %v", err)
	}
	recent, err := d.RecentRuns(ctx, 10)
	if err != nil {
		t.Fatalf("RecentRuns: %v", err)
	}
	if len(recent) != 3 {
		t.Fatalf("expected 3 rows after prune, got %d", len(recent))
	}
}
