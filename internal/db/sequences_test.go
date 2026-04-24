package db

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

func TestChainPersistenceRoundTrip(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()

	ctx := context.Background()
	cmds := []string{"build", "test", "lint"}

	seqID, err := d.UpsertChainSequence(ctx, cmds)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if seqID == 0 {
		t.Fatal("seqID zero")
	}

	// Repeat upsert dedupes.
	seqID2, err := d.UpsertChainSequence(ctx, cmds)
	if err != nil {
		t.Fatalf("upsert2: %v", err)
	}
	if seqID2 != seqID {
		t.Fatalf("dedup failed: %d != %d", seqID2, seqID)
	}

	start := time.Now()
	srID, err := d.StartSequenceRun(ctx, seqID, start)
	if err != nil {
		t.Fatalf("start seq run: %v", err)
	}

	// Insert per-step runs and link them.
	for i, c := range cmds {
		r := NewRun(c, "local", start, time.Second, nil, false)
		rid, err := d.InsertRun(ctx, r)
		if err != nil {
			t.Fatalf("insert run %d: %v", i, err)
		}
		_, err = d.SQL().ExecContext(ctx,
			`UPDATE runs SET sequence_run_id = ?, step_index = ? WHERE id = ?`,
			srID, i+1, rid)
		if err != nil {
			t.Fatalf("link run %d: %v", i, err)
		}
	}
	if err := d.FinishSequenceRun(ctx, srID, StatusSuccess, time.Now(), 3*time.Second); err != nil {
		t.Fatalf("finish seq: %v", err)
	}

	summaries, err := d.ListChainSummaries(ctx, 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("want 1 summary got %d", len(summaries))
	}
	s := summaries[0]
	if got, want := s.Commands, cmds; !eqStrings(got, want) {
		t.Errorf("commands got %v want %v", got, want)
	}
	if s.RunCount != 1 {
		t.Errorf("run count got %d want 1", s.RunCount)
	}
	if !s.LastStatus.Valid || RunStatus(s.LastStatus.Int64) != StatusSuccess {
		t.Errorf("last status got %+v want success", s.LastStatus)
	}

	// Second execution of same chain → run_count == 2.
	srID2, err := d.StartSequenceRun(ctx, seqID, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := d.FinishSequenceRun(ctx, srID2, StatusError, time.Now(), time.Second); err != nil {
		t.Fatal(err)
	}
	summaries, _ = d.ListChainSummaries(ctx, 10)
	if summaries[0].RunCount != 2 {
		t.Errorf("after 2nd: got %d want 2", summaries[0].RunCount)
	}
	if RunStatus(summaries[0].LastStatus.Int64) != StatusError {
		t.Errorf("want last=error, got %+v", summaries[0].LastStatus)
	}
}

func eqStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestListChainSummariesEmpty(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()
	out, err := d.ListChainSummaries(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 {
		t.Errorf("want empty, got %d", len(out))
	}
	_ = sql.NullInt64{} // satisfy import
}
