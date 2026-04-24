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

// StreamRun starts "make <target>" in a goroutine and returns a channel that
// emits OutputMsg for each stdout/stderr line and a final DoneMsg when done.
// The channel is closed after DoneMsg is sent.
func StreamRun(target string) <-chan tea.Msg {
	ch := make(chan tea.Msg, 32)
	go func() {
		start := time.Now()
		cmd := exec.Command("make", target)

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			ch <- DoneMsg{Err: err}
			close(ch)
			return
		}
		cmd.Stderr = cmd.Stdout

		if err := cmd.Start(); err != nil {
			ch <- DoneMsg{Err: err}
			close(ch)
			return
		}

		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			ch <- OutputMsg{Line: scanner.Text()}
		}

		ch <- DoneMsg{Err: cmd.Wait(), Duration: time.Since(start)}
		close(ch)
	}()
	return ch
}
