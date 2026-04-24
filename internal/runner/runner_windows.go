//go:build windows

package runner

import "os/exec"

// configureProcess is a no-op on Windows; exec.CommandContext already handles
// cancellation via Process.Kill(), which kills the immediate child.
func configureProcess(cmd *exec.Cmd) {}

// interruptProcess attempts a graceful kill. Windows lacks SIGINT for console
// subprocesses spawned by Go without extra setup, so we fall back to Kill().
func interruptProcess(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
}

func killProcess(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
}
