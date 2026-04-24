//go:build !windows

package runner

import (
	"os/exec"
	"syscall"
)

// configureProcess puts the child in its own process group so we can signal
// the whole tree (make + its spawned subprocesses) at once.
func configureProcess(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

// interruptProcess sends SIGINT to the whole process group.
func interruptProcess(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		_ = cmd.Process.Signal(syscall.SIGINT)
		return
	}
	_ = syscall.Kill(-pgid, syscall.SIGINT)
}

// killProcess force-kills the whole process group.
func killProcess(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		_ = cmd.Process.Kill()
		return
	}
	_ = syscall.Kill(-pgid, syscall.SIGKILL)
}
