package session

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/platform"
	"github.com/hironow/sightjack/internal/usecase/port"
)

// sharedCircuitBreaker is the process-wide circuit breaker shared across all
// provider adapter instances. Set via SetCircuitBreaker at startup.
var sharedCircuitBreaker *platform.CircuitBreaker

// SetCircuitBreaker sets the process-wide circuit breaker for all provider calls.
// Call this once during startup before any provider invocations.
func SetCircuitBreaker(cb *platform.CircuitBreaker) {
	sharedCircuitBreaker = cb
}

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

// WithWorkDir sets the working directory for the Claude subprocess.
var WithWorkDir = port.WithWorkDir

// WithConfigBase sets the base directory for resolving stateDir settings.
var WithConfigBase = port.WithConfigBase

// NewClaudeAdapter creates a ClaudeAdapter implementing port.ClaudeRunner.
func NewClaudeAdapter(cfg *domain.Config, logger domain.Logger) *ClaudeAdapter {
	return &ClaudeAdapter{
		ClaudeCmd:  cfg.ClaudeCmd,
		Model:      cfg.Model,
		TimeoutSec: cfg.TimeoutSec,
		Logger:     logger,
		NewCmd:     newCmd,
		CancelFunc: cancelFunc,
	}
}

// NewRetryRunner creates a RetryRunner wrapping the given ClaudeRunner.
func NewRetryRunner(inner port.ClaudeRunner, cfg *domain.Config, logger domain.Logger) *RetryRunner {
	return &RetryRunner{
		Inner:          inner,
		MaxAttempts:    cfg.Retry.MaxAttempts,
		BaseDelay:      time.Duration(cfg.Retry.BaseDelaySec) * time.Second,
		Timeout:        time.Duration(cfg.TimeoutSec) * time.Second,
		Logger:         logger,
		CircuitBreaker: sharedCircuitBreaker,
	}
}

// NewTrackedRunner creates a provider-tracked runner with retry, session tracking,
// and circuit breaker protection. This is the standard way to create a runner
// for session-level operations that should be retried on transient failures.
// baseDir is used to resolve the session tracking database path.
func NewTrackedRunner(cfg *domain.Config, baseDir string, logger domain.Logger) port.ClaudeRunner {
	adapter := NewClaudeAdapter(cfg, logger)
	retrier := NewRetryRunner(adapter, cfg, logger)
	return wrapWithSessionTracking(retrier, baseDir, logger)
}

// NewOnceRunner creates a provider-tracked runner WITHOUT retry.
// Use this for operations with side-effects that must not be retried
// (e.g. wave apply, classify with label mutations).
// baseDir is used to resolve the session tracking database path.
func NewOnceRunner(cfg *domain.Config, baseDir string, logger domain.Logger) port.ClaudeRunner {
	adapter := NewClaudeAdapter(cfg, logger)
	return wrapWithSessionTracking(adapter, baseDir, logger)
}

// wrapWithSessionTracking wraps a ClaudeRunner with session persistence.
// Best-effort: if the session store cannot be opened, returns the runner as-is.
func wrapWithSessionTracking(runner port.ClaudeRunner, baseDir string, logger domain.Logger) port.ClaudeRunner {
	detailed, ok := runner.(port.DetailedRunner)
	if !ok {
		return runner
	}
	dbPath := filepath.Join(baseDir, domain.StateDir, ".run", "sessions.db")
	store, err := NewSQLiteCodingSessionStore(dbPath)
	if err != nil {
		if logger != nil {
			logger.Debug("session tracking unavailable: %v", err)
		}
		return runner
	}
	// Note: store is not closed here — it lives for the duration of the session.
	// The caller (RunSession/RunResumeSession) should hold the runner reference.
	return NewSessionTrackingAdapter(detailed, store, domain.ProviderClaudeCode)
}

// recordCircuitBreaker updates the shared circuit breaker based on provider error classification.
func recordCircuitBreaker(provider domain.Provider, err error, stderr string) {
	if sharedCircuitBreaker == nil {
		return
	}
	if err == nil {
		sharedCircuitBreaker.RecordSuccess()
		return
	}
	// Use stderr if available, otherwise try extracting from the error message itself
	classifyTarget := stderr
	if classifyTarget == "" {
		classifyTarget = err.Error()
	}
	info := domain.ClassifyProviderError(provider, classifyTarget)
	if info.IsTrip() {
		sharedCircuitBreaker.RecordProviderError(info)
	}
}

// RunClaudeDryRun saves the prompt to a file instead of executing the provider CLI,
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

// AllowedToolsForMode returns the appropriate tool list based on tracking mode.
// Wave mode: base + GitHub tools only (no Linear MCP).
// Linear mode: base + GitHub + Linear MCP tools.
func AllowedToolsForMode(mode domain.TrackingMode) []string {
	tools := make([]string, 0, len(BaseAllowedTools)+len(GHAllowedTools)+len(LinearMCPAllowedTools))
	tools = append(tools, BaseAllowedTools...)
	tools = append(tools, GHAllowedTools...)
	if mode.IsLinear() {
		tools = append(tools, LinearMCPAllowedTools...)
	}
	return tools
}
