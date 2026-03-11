package session

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/platform"
	"github.com/hironow/sightjack/internal/usecase/port"
)

var newCmd = defaultNewCmd

func defaultNewCmd(ctx context.Context, name string, args ...string) *exec.Cmd {
	return platform.NewShellCmd(ctx, name, args...)
}

// OverrideNewCmd replaces the command constructor for testing and returns a
// cleanup function. Exported for cross-package test injection (root test suite).
func OverrideNewCmd(fn func(ctx context.Context, name string, args ...string) *exec.Cmd) func() {
	old := newCmd
	newCmd = fn
	return func() { newCmd = old }
}

var lookPath = platform.LookPathShell

// OverrideLookPath replaces the path lookup function for testing and returns a
// cleanup function.
func OverrideLookPath(fn func(cmd string) (string, error)) func() {
	old := lookPath
	lookPath = fn
	return func() { lookPath = old }
}

// RunOption is an alias for port.RunOption for backward compatibility.
type RunOption = port.RunOption

// WithAllowedTools restricts the tools available to the Claude model.
var WithAllowedTools = port.WithAllowedTools

// NewClaudeAdapter creates a ClaudeAdapter implementing port.ClaudeRunner.
func NewClaudeAdapter(cfg *domain.Config, logger domain.Logger) *ClaudeAdapter {
	return &ClaudeAdapter{Cfg: cfg, Logger: logger}
}

// NewRetryRunner creates a RetryRunner wrapping the given ClaudeRunner.
func NewRetryRunner(inner port.ClaudeRunner, cfg *domain.Config, logger domain.Logger) *RetryRunner {
	return &RetryRunner{
		Inner:   inner,
		Retry:   cfg.Retry,
		Timeout: time.Duration(cfg.TimeoutSec) * time.Second,
		Logger:  logger,
	}
}

// RunClaudeOnce executes the Claude CLI as a subprocess once without retry.
// Thin wrapper around ClaudeAdapter for call-site compatibility.
func RunClaudeOnce(ctx context.Context, cfg *domain.Config, prompt string, w io.Writer, logger domain.Logger, opts ...port.RunOption) (string, error) {
	return NewClaudeAdapter(cfg, logger).Run(ctx, prompt, w, opts...)
}

// RunClaude executes the Claude CLI with exponential backoff retry.
// Thin wrapper around RetryRunner{ClaudeAdapter} for call-site compatibility.
func RunClaude(ctx context.Context, cfg *domain.Config, prompt string, w io.Writer, logger domain.Logger, opts ...port.RunOption) (string, error) {
	adapter := NewClaudeAdapter(cfg, logger)
	return NewRetryRunner(adapter, cfg, logger).Run(ctx, prompt, w, opts...)
}

// RunClaudeDryRun saves the prompt to a file instead of executing Claude,
// useful for previewing what would be sent. The name parameter makes each
// prompt file unique within the output directory (e.g. "classify", "wave_00_auth").
func RunClaudeDryRun(cfg *domain.Config, prompt, outputPath string, name string, logger domain.Logger) error {
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
// are generally useful and safe to enable by default for most workflows.
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
