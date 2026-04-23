package runner

import (
	"bufio"
	"os/exec"
	"time"

	tea "charm.land/bubbletea/v2"
)

// OutputMsg carries a single streamed stdout/stderr line.
type OutputMsg struct{ Line string }

// DoneMsg signals that the command has finished.
type DoneMsg struct {
	Err      error
	Duration time.Duration
}

// Run executes "make <target>" and returns a DoneMsg when finished.
// For streaming output use StreamRun.
func Run(target string) tea.Cmd {
	return func() tea.Msg {
		start := time.Now()
		cmd := exec.Command("make", target)
		if err := cmd.Run(); err != nil {
			return DoneMsg{Err: err, Duration: time.Since(start)}
		}
		return DoneMsg{Duration: time.Since(start)}
	}
}

// StreamRun starts "make <target>" in a goroutine and calls send for every
// output line and once more when the process exits.
func StreamRun(target string, send func(tea.Msg)) {
	go func() {
		start := time.Now()
		cmd := exec.Command("make", target)

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			send(DoneMsg{Err: err})
			return
		}
		cmd.Stderr = cmd.Stdout

		if err := cmd.Start(); err != nil {
			send(DoneMsg{Err: err})
			return
		}

		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			send(OutputMsg{Line: scanner.Text()})
		}

		err = cmd.Wait()
		send(DoneMsg{Err: err, Duration: time.Since(start)})
	}()
}
