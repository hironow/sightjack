package sightjack

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// Notifier sends a notification to the user.
type Notifier interface {
	Notify(ctx context.Context, title, message string) error
}

// cmdFactoryFunc creates an *exec.Cmd — injectable for testing.
type cmdFactoryFunc func(ctx context.Context, name string, args ...string) *exec.Cmd

func defaultCmdFactory(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}

// NopNotifier is a no-op notifier for tests and quiet mode.
type NopNotifier struct{}

func (n *NopNotifier) Notify(_ context.Context, _, _ string) error { return nil }

// LocalNotifier sends desktop notifications using OS-native tools.
// darwin: osascript with Funk sound, linux: notify-send.
type LocalNotifier struct {
	forceOS    string         // override runtime.GOOS for testing
	cmdFactory cmdFactoryFunc // override exec.CommandContext for testing
}

func (n *LocalNotifier) os() string {
	if n.forceOS != "" {
		return n.forceOS
	}
	return runtime.GOOS
}

func (n *LocalNotifier) factory() cmdFactoryFunc {
	if n.cmdFactory != nil {
		return n.cmdFactory
	}
	return defaultCmdFactory
}

func (n *LocalNotifier) Notify(ctx context.Context, title, message string) error {
	factory := n.factory()
	switch n.os() {
	case "darwin":
		script := fmt.Sprintf(`display notification %q with title %q sound name "Funk"`, message, title)
		cmd := factory(ctx, "osascript", "-e", script)
		return cmd.Run()
	case "linux":
		cmd := factory(ctx, "notify-send", title, message)
		return cmd.Run()
	default:
		return fmt.Errorf("notify: unsupported OS %q", n.os())
	}
}

// CmdNotifier runs a user-provided shell command with {title} and {message} placeholders.
type CmdNotifier struct {
	template   string
	cmdFactory cmdFactoryFunc
}

// NewCmdNotifier creates a CmdNotifier from a shell command template.
func NewCmdNotifier(template string) *CmdNotifier {
	return &CmdNotifier{template: template}
}

func (n *CmdNotifier) factory() cmdFactoryFunc {
	if n.cmdFactory != nil {
		return n.cmdFactory
	}
	return defaultCmdFactory
}

func (n *CmdNotifier) Notify(ctx context.Context, title, message string) error {
	if n.template == "" {
		return fmt.Errorf("notify: empty command template")
	}
	expanded := strings.ReplaceAll(n.template, "{title}", ShellQuote(title))
	expanded = strings.ReplaceAll(expanded, "{message}", ShellQuote(message))
	cmd := n.factory()(ctx, "sh", "-c", expanded)
	return cmd.Run()
}

// ShellQuote wraps a string in single quotes with proper escaping
// to prevent shell injection. Single quotes within the string are
// escaped by splitting: quote -> quote-backslash-quote-quote (see implementation).
func ShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
