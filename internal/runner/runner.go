package runner

import (
	"bufio"
	"context"
	"errors"
	"os/exec"
	"time"

	tea "charm.land/bubbletea/v2"
)

// OutputMsg carries a single streamed stdout/stderr line.
type OutputMsg struct{ Line string }

// DoneMsg signals that the command has finished.
type DoneMsg struct {
	Err         error
	Duration    time.Duration
	Interrupted bool // true when the run was canceled via ctx (user stop)
}

// Run executes "make <target>" and returns a DoneMsg when finished.
// For streaming output use StreamRun.
func Run(ctx context.Context, target string) tea.Cmd {
	return func() tea.Msg {
		start := time.Now()
		cmd := exec.CommandContext(ctx, "make", target)
		configureProcess(cmd)
		err := cmd.Run()
		return DoneMsg{
			Err:         err,
			Duration:    time.Since(start),
			Interrupted: errors.Is(ctx.Err(), context.Canceled),
		}
	}
}

// StreamRun starts "make <target>" in a goroutine and returns a channel that
// emits OutputMsg for each stdout/stderr line and a final DoneMsg when done.
// The channel is closed after DoneMsg is sent.
//
// ctx cancellation sends SIGINT to the subprocess' process group so child
// processes (e.g. `docker logs -f` spawned by make) also terminate. If the
// process does not exit within 2s, SIGKILL is sent.
func StreamRun(ctx context.Context, target string) <-chan tea.Msg {
	ch := make(chan tea.Msg, 128)
	go func() {
		start := time.Now()
		cmd := exec.Command("make", target)
		configureProcess(cmd)

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

		// Watchdog: on ctx cancel, gracefully terminate the process group.
		killed := make(chan struct{})
		go func() {
			select {
			case <-ctx.Done():
				interruptProcess(cmd)
				select {
				case <-killed:
				case <-time.After(2 * time.Second):
					killProcess(cmd)
				}
			case <-killed:
			}
		}()

		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			ch <- OutputMsg{Line: scanner.Text()}
		}

		waitErr := cmd.Wait()
		close(killed)
		ch <- DoneMsg{
			Err:         waitErr,
			Duration:    time.Since(start),
			Interrupted: errors.Is(ctx.Err(), context.Canceled),
		}
		close(ch)
	}()
	return ch
}
