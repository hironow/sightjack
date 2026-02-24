package session

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

// NewLocalNotifierForTest creates a LocalNotifier with test overrides.
func NewLocalNotifierForTest(osName string, factory func(ctx context.Context, name string, args ...string) *exec.Cmd) *LocalNotifier {
	return &LocalNotifier{forceOS: osName, cmdFactory: factory}
}

// NewCmdNotifierForTest creates a CmdNotifier with a test command factory.
func NewCmdNotifierForTest(template string, factory func(ctx context.Context, name string, args ...string) *exec.Cmd) *CmdNotifier {
	return &CmdNotifier{template: template, cmdFactory: factory}
}

// NewCmdApproverForTest creates a CmdApprover with a test command factory.
func NewCmdApproverForTest(template string, factory func(ctx context.Context, name string, args ...string) *exec.Cmd) *CmdApprover {
	return &CmdApprover{template: template, cmdFactory: factory}
}
