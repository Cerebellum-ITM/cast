package db

import (
	"context"
	"errors"
	"time"
)

// ErrNotImplemented is returned by sequence APIs that exist as forward-plan
// stubs. The schema for sequences already lives in 001_init.sql so a future
// implementation can land without a new migration.
var ErrNotImplemented = errors.New("db: sequences not implemented yet")

// Sequence is the definition of a reusable named sequence of commands.
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

// SequenceRun is a single execution of a sequence (named or ad-hoc).
type SequenceRun struct {
	ID          int64
	SequenceID  int64 // 0 = ad-hoc
	Name        string
	Status      RunStatus
	StartedAt   time.Time
	FinishedAt  time.Time
	DurationMs  int64
}

// CreateSequence is a forward-plan stub.
func (d *DB) CreateSequence(ctx context.Context, s Sequence) (int64, error) {
	return 0, ErrNotImplemented
}

// ListSequences is a forward-plan stub.
func (d *DB) ListSequences(ctx context.Context) ([]Sequence, error) {
	return nil, ErrNotImplemented
}

// SequenceSteps is a forward-plan stub.
func (d *DB) SequenceSteps(ctx context.Context, sequenceID int64) ([]SequenceStep, error) {
	return nil, ErrNotImplemented
}

// StartSequenceRun is a forward-plan stub.
func (d *DB) StartSequenceRun(ctx context.Context, sr SequenceRun) (int64, error) {
	return 0, ErrNotImplemented
}

// FinishSequenceRun is a forward-plan stub.
func (d *DB) FinishSequenceRun(ctx context.Context, id int64, status RunStatus, finishedAt time.Time) error {
	return ErrNotImplemented
}
