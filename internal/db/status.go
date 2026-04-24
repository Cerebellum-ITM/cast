package db

// RunStatus represents the outcome of a command execution.
// Values are persisted to SQLite, so they are stable integers.
type RunStatus int

const (
	StatusSuccess RunStatus = 0
	StatusError   RunStatus = 1
	StatusRunning RunStatus = 2
)
