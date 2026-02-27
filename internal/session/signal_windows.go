//go:build windows

package session

import "os/exec"

// cancelFunc returns a function that terminates the process immediately.
// On Windows, there is no SIGINT equivalent for non-console processes, so
// we fall back to process termination. Used as exec.Cmd.Cancel.
func cancelFunc(cmd *exec.Cmd) func() error {
	return func() error { return cmd.Process.Kill() }
}
