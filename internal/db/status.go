package db

// RunStatus represents the outcome of a command execution.
// Values are persisted to SQLite, so they are stable integers.
type RunStatus int

const (
	StatusSuccess     RunStatus = 0
	StatusError       RunStatus = 1
	StatusRunning     RunStatus = 2
	StatusInterrupted RunStatus = 3
)

// String returns a short human-readable label used by the TUI.
func (s RunStatus) String() string {
	switch s {
	case StatusSuccess:
		return "ok"
	case StatusError:
		return "err"
	case StatusRunning:
		return "running"
	case StatusInterrupted:
		return "stopped"
	default:
		return "?"
	}
}
