package session

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	sightjack "github.com/hironow/sightjack"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var newCmd = defaultNewCmd

func defaultNewCmd(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}

// OverrideNewCmd replaces the command constructor for testing and returns a
// cleanup function. Exported for cross-package test injection (root test suite).
func OverrideNewCmd(fn func(ctx context.Context, name string, args ...string) *exec.Cmd) func() {
	old := newCmd
	newCmd = fn
	return func() { newCmd = old }
}

// RunOption configures optional behavior for RunClaudeOnce / RunClaude.
type RunOption func(*runOpts)

type runOpts struct {
	allowedTools []string
}

// WithAllowedTools restricts the tools available to the Claude model via
// the --allowedTools CLI flag. This prevents context explosion when many
// plugins/MCP servers are loaded (see anthropics/claude-code#25857).
func WithAllowedTools(tools ...string) RunOption {
	return func(o *runOpts) {
		o.allowedTools = tools
	}
}

// RunClaudeOnce executes the Claude CLI as a subprocess once without retry.
// Use this for prompts that perform non-idempotent mutations (e.g. applying
// labels or updating descriptions via Linear MCP) where retrying after a
// partial success could duplicate side effects.
func RunClaudeOnce(ctx context.Context, cfg *sightjack.Config, prompt string, w io.Writer, logger *sightjack.Logger, opts ...RunOption) (string, error) {
	ctx, span := tracer.Start(ctx, "claude.invoke",
		trace.WithAttributes(
			attribute.String("claude.model", cfg.Claude.Model),
			attribute.Int("claude.timeout_sec", cfg.Claude.TimeoutSec),
		),
	)
	defer span.End()

	// Apply per-call timeout only when the caller has not already set a deadline.
	// RunClaude wraps the entire retry loop in a single timeout, so individual
	// attempts inherit the remaining budget without resetting it.
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		timeout := time.Duration(cfg.Claude.TimeoutSec) * time.Second
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	var o runOpts
	for _, opt := range opts {
		opt(&o)
	}

	var args []string
	if cfg.Claude.Model != "" {
		args = append(args, "--model", cfg.Claude.Model)
	}
	if len(o.allowedTools) > 0 {
		args = append(args, "--allowedTools", strings.Join(o.allowedTools, ","))
	}
	args = append(args, "--dangerously-skip-permissions", "--print", "-p", prompt)
	cmd := newCmd(ctx, cfg.Claude.Command, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("stdout pipe: %w", err)
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("claude start: %w", err)
	}

	var output strings.Builder
	done := make(chan struct{})

	go func() {
		defer close(done)
		reader := bufio.NewReader(stdout)
		buf := make([]byte, 4096)
		for {
			n, readErr := reader.Read(buf)
			if n > 0 {
				chunk := buf[:n]
				_, _ = w.Write(chunk)
				output.Write(chunk)
			}
			if readErr != nil {
				if readErr != io.EOF {
					if logger != nil {
						logger.Warn("stdout read: %v", readErr)
					}
				}
				break
			}
		}
	}()

	<-done

	if err := cmd.Wait(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			span.AddEvent("claude.timeout",
				trace.WithAttributes(attribute.Int("claude.timeout_sec", cfg.Claude.TimeoutSec)),
			)
		}
		return output.String(), fmt.Errorf("claude exit: %w", err)
	}

	return output.String(), nil
}

// RunClaude executes the Claude CLI as a subprocess with exponential backoff
// retry. It streams output to w in real time and returns the full output when
// complete.
// Pass os.Stdout for interactive single-process usage, or io.Discard for
// parallel invocations where interleaved output would be unreadable.
func RunClaude(ctx context.Context, cfg *sightjack.Config, prompt string, w io.Writer, logger *sightjack.Logger, opts ...RunOption) (string, error) {
	maxAttempts := cfg.Retry.MaxAttempts
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	baseDelay := time.Duration(cfg.Retry.BaseDelaySec) * time.Second

	// Wrap the entire retry loop in a single timeout so total wall time
	// is bounded by TimeoutSec regardless of MaxAttempts.
	timeout := time.Duration(cfg.Claude.TimeoutSec) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if attempt > 1 {
			shift := attempt - 2
			if shift > 30 {
				shift = 30 // cap to prevent overflow of time.Duration
			}
			delay := baseDelay * time.Duration(1<<shift) // exponential: base*2^0, base*2^1, base*2^2...
			logger.Info("Retrying (%d/%d) after %v...", attempt, maxAttempts, delay)
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(delay):
			}
		}
		output, err := RunClaudeOnce(ctx, cfg, prompt, w, logger, opts...)
		if err == nil {
			return output, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return output, err
		}
		// Record retry event on the current span (if any).
		span := trace.SpanFromContext(ctx)
		span.AddEvent("claude.retry",
			trace.WithAttributes(
				attribute.Int("claude.attempt", attempt),
				attribute.String("claude.error", err.Error()),
			),
		)
	}
	return "", fmt.Errorf("claude failed after %d attempts: %w", maxAttempts, lastErr)
}

// RunClaudeDryRun saves the prompt to a file instead of executing Claude,
// useful for previewing what would be sent. The name parameter makes each
// prompt file unique within the output directory (e.g. "classify", "wave_00_auth").
func RunClaudeDryRun(cfg *sightjack.Config, prompt, outputPath string, name string, logger *sightjack.Logger) error {
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		return fmt.Errorf("create dry-run dir: %w", err)
	}
	promptFile := filepath.Join(outputPath, name+"_prompt.md")
	if err := os.WriteFile(promptFile, []byte(prompt), 0644); err != nil {
		return fmt.Errorf("write prompt: %w", err)
	}
	logger.Info("Dry-run: prompt saved to %s", promptFile)
	return nil
}

// Base tools (e.g. filesystem access, basic bash) that
// are generally useful and safe to
var BaseAllowedTools = []string{
	"Read",
	"Write",
	"Bash(ls:*)",
	"Bash(wc:*)",
	"Bash(find:*)",
	"Bash(echo:*)",
	"Bash(sort:*)",
	"Bash(grep:*)",
	"Bash(awk:*)",
	"Bash(sed:*)",
	"Bash(head:*)",
	"Bash(cat:*)",
}

// GitHub CLI tools (git, gh) and GitHub WebFetch tools that
// sightjack commonly uses for GitHub-related tasks.
var GHAllowedTools = []string{
	"Bash(git:*)",
	"Bash(gh:*)",
	"WebFetch(domain:github.com)",
	"WebFetch(domain:raw.githubusercontent.com)",
}

// LinearMCPAllowedTools lists the official Linear MCP server tools that
// sightjack needs. Passing this via WithAllowedTools prevents context
// explosion from unrelated plugins loading hundreds of tool definitions
// (see anthropics/claude-code#25857).
var LinearMCPAllowedTools = []string{
	"mcp__linear__list_issues",
	"mcp__linear__get_issue",
	"mcp__linear__create_issue",
	"mcp__linear__update_issue",
	"mcp__linear__list_issue_statuses",
	"mcp__linear__get_issue_status",
	"mcp__linear__list_issue_labels",
	"mcp__linear__create_issue_label",
	"mcp__linear__list_comments",
	"mcp__linear__create_comment",
	"mcp__linear__list_projects",
	"mcp__linear__get_project",
	"mcp__linear__save_project",
	"mcp__linear__list_project_labels",
	"mcp__linear__list_teams",
	"mcp__linear__get_team",
	"mcp__linear__list_users",
	"mcp__linear__get_user",
	"mcp__linear__list_cycles",
	// "mcp__linear__list_documents",
	// "mcp__linear__get_document",
	// "mcp__linear__create_document",
	// "mcp__linear__update_document",
	// "mcp__linear__list_milestones",
	// "mcp__linear__get_milestone",
	// "mcp__linear__create_milestone",
	// "mcp__linear__update_milestone",
	// "mcp__linear__get_attachment",
	// "mcp__linear__create_attachment",
	// "mcp__linear__delete_attachment",
	// "mcp__linear__extract_images",
	"mcp__linear__search_documentation",
}
