package runner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// RunStatus represents the outcome of a command execution.
type RunStatus int

const (
	StatusSuccess RunStatus = iota
	StatusError
	StatusRunning
)

// RunRecord is a single entry in the run history.
type RunRecord struct {
	Command  string    `json:"command"`
	Duration string    `json:"duration"`
	Status   RunStatus `json:"status"`
	Time     string    `json:"time"`
}

// NewRecord creates a RunRecord from execution results.
func NewRecord(command string, dur time.Duration, err error) RunRecord {
	status := StatusSuccess
	if err != nil {
		status = StatusError
	}
	return RunRecord{
		Command:  command,
		Duration: formatDuration(dur),
		Status:   status,
		Time:     time.Now().Format("15:04:05"),
	}
}

// LoadHistory reads persisted run history from path.
// Returns nil if the file does not exist.
func LoadHistory(path string) ([]RunRecord, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var records []RunRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, err
	}
	return records, nil
}

// SaveHistory writes records to path as JSON, trimming to max entries.
func SaveHistory(path string, records []RunRecord, max int) error {
	if len(records) > max {
		records = records[:max]
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return d.Truncate(time.Millisecond).String()
	}
	return d.Truncate(time.Second).String()
}
