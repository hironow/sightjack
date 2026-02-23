package sightjack

import (
	"context"
	"os/exec"
)

// SetNewCmd replaces the command constructor for testing and returns a cleanup function.
// This is test infrastructure for injecting fake commands, not a logic shim.
func SetNewCmd(fn func(ctx context.Context, name string, args ...string) *exec.Cmd) func() {
	old := newCmd
	newCmd = fn
	return func() { newCmd = old }
}
