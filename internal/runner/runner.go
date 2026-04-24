package runner

import (
	"bufio"
	"context"
	"errors"
	"os"
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
// When dir != "", make is invoked with `-C <dir>` so recipes evaluate from
// the directory that holds the Makefile (useful for monorepos / submodules
// where cast was launched from a subdirectory). For streaming output use
// StreamRun.
func Run(ctx context.Context, dir, target string) tea.Cmd {
	return func() tea.Msg {
		start := time.Now()
		cmd := exec.CommandContext(ctx, "make", makeArgs(dir, target)...)
		configureProcess(cmd)
		err := cmd.Run()
		return DoneMsg{
			Err:         err,
			Duration:    time.Since(start),
			Interrupted: errors.Is(ctx.Err(), context.Canceled),
		}
	}
}

// makeArgs prepends `-C <dir>` when dir is set so recipes run relative to
// the Makefile's directory. Empty dir preserves the legacy cwd behavior.
func makeArgs(dir, target string) []string {
	if dir == "" {
		return []string{target}
	}
	return []string{"-C", dir, target}
}

// InteractiveRun returns a tea.Cmd that runs "make <target>" with the user's
// real TTY attached (stdin/stdout/stderr inherited). The Bubble Tea program is
// paused while the command runs — the alt-screen is released so editors and
// REPLs (python3, psql, bash, vim…) can take over the terminal — and resumed
// once the process exits. A DoneMsg is emitted through the Program on finish.
func InteractiveRun(dir, target string) tea.Cmd {
	start := time.Now()
	cmd := exec.Command("make", makeArgs(dir, target)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return DoneMsg{
			Err:      err,
			Duration: time.Since(start),
		}
	})
}

// StreamRun starts "make <target>" in a goroutine and returns a channel that
// emits OutputMsg for each stdout/stderr line and a final DoneMsg when done.
// The channel is closed after DoneMsg is sent.
//
// ctx cancellation sends SIGINT to the subprocess' process group so child
// processes (e.g. `docker logs -f` spawned by make) also terminate. If the
// process does not exit within 2s, SIGKILL is sent.
func StreamRun(ctx context.Context, dir, target string) <-chan tea.Msg {
	ch := make(chan tea.Msg, 128)
	go func() {
		start := time.Now()
		cmd := exec.Command("make", makeArgs(dir, target)...)
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
