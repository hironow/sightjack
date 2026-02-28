package session

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// cmdFactoryFunc creates an *exec.Cmd — injectable for testing.
type cmdFactoryFunc func(ctx context.Context, name string, args ...string) *exec.Cmd

func defaultCmdFactory(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}

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
	case "windows":
		script := fmt.Sprintf(
			`Add-Type -AssemblyName System.Windows.Forms; `+
				`$n = New-Object System.Windows.Forms.NotifyIcon; `+
				`$n.Icon = [System.Drawing.SystemIcons]::Information; `+
				`$n.BalloonTipTitle = '%s'; `+
				`$n.BalloonTipText = '%s'; `+
				`$n.Visible = $true; `+
				`$n.ShowBalloonTip(5000)`,
			psEscapeSingleQuote(title), psEscapeSingleQuote(message),
		)
		cmd := factory(ctx, "powershell", "-NoProfile", "-Command", script)
		return cmd.Run()
	default:
		return fmt.Errorf("notify: unsupported OS %q", n.os())
	}
}

// psEscapeSingleQuote escapes single quotes for PowerShell single-quoted strings.
func psEscapeSingleQuote(s string) string {
	return strings.ReplaceAll(s, "'", "''")
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
	cmd := n.factory()(ctx, shellName(), shellFlag(), expanded)
	return cmd.Run()
}

