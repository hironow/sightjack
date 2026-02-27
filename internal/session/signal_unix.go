//go:build !windows

package session

import (
	"os/exec"
	"syscall"
)

// cancelFunc returns a function that sends SIGINT to the process for graceful
// shutdown. On Unix systems, SIGINT allows the subprocess to clean up before
// exiting. Used as exec.Cmd.Cancel to implement 2-stage shutdown:
// SIGINT → WaitDelay → SIGKILL.
func cancelFunc(cmd *exec.Cmd) func() error {
	return func() error { return cmd.Process.Signal(syscall.SIGINT) }
}
