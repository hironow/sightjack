package session

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/eventsource"
)

// CheckConfig validates that the config file exists and can be loaded.
func CheckConfig(configPath string) domain.CheckResult {
	_, err := LoadConfig(configPath)
	if err != nil {
		return domain.CheckResult{
			Name:    "Config",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("%s: %v", configPath, err),
			Hint:    `run "sightjack init --team <TEAM> --project <PROJECT>" to create a config file`,
		}
	}
	return domain.CheckResult{
		Name:    "Config",
		Status:  domain.CheckOK,
		Message: fmt.Sprintf("%s loaded successfully", configPath),
	}
}

// CheckTool verifies that a CLI tool is installed and executable.
// It runs `<tool> --version` to confirm functionality.
func CheckTool(ctx context.Context, name string) domain.CheckResult {
	path, err := exec.LookPath(name)
	if err != nil {
		return domain.CheckResult{
			Name:    name,
			Status:  domain.CheckFail,
			Message: "command not found",
			Hint:    fmt.Sprintf("install %s and ensure it is in PATH", name),
		}
	}

	out, err := exec.CommandContext(ctx, path, "--version").Output()
	if err != nil {
		return domain.CheckResult{
			Name:    name,
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("found at %s but --version failed: %v", path, err),
			Hint:    fmt.Sprintf("%s may be corrupted; reinstall it", name),
		}
	}

	version := strings.TrimSpace(strings.Split(string(out), "\n")[0])
	return domain.CheckResult{
		Name:    name,
		Status:  domain.CheckOK,
		Message: fmt.Sprintf("%s (%s)", path, version),
	}
}

// CheckClaudeAuth verifies that Claude Code is authenticated by sending a
// simple prompt that does not require any MCP server.
// Returns CheckSkip if cfg is nil (config loading failed).
func CheckClaudeAuth(ctx context.Context, cfg *domain.Config, logger domain.Logger) domain.CheckResult {
	if cfg == nil {
		return domain.CheckResult{
			Name:    "Claude Auth",
			Status:  domain.CheckSkip,
			Message: "skipped (config not available)",
		}
	}

	output, err := RunClaudeOnce(ctx, cfg, "Reply with only the word OK.", io.Discard, logger, WithAllowedTools("Write"))
	if err != nil {
		hint := fmt.Sprintf("claude execution failed: %v", err)
		if strings.Contains(output, "Not logged in") {
			return domain.CheckResult{
				Name:    "Claude Auth",
				Status:  domain.CheckFail,
				Message: "not logged in",
				Hint:    `run "claude login" then "/login" inside the session (in Docker: set CLAUDE_CONFIG_DIR=~/.claude to use host credentials)`,
			}
		}
		return domain.CheckResult{
			Name:    "Claude Auth",
			Status:  domain.CheckFail,
			Message: hint,
			Hint:    `check Claude CLI with "claude --version"; if auth issue, run "claude login" (in Docker: set CLAUDE_CONFIG_DIR=~/.claude to use host credentials)`,
		}
	}

	return domain.CheckResult{
		Name:    "Claude Auth",
		Status:  domain.CheckOK,
		Message: "authenticated",
	}
}

// CheckLinearMCP verifies Linear MCP connectivity by sending a prompt that
// references the configured Linear team.
// Returns CheckSkip if cfg is nil (config loading failed).
func CheckLinearMCP(ctx context.Context, cfg *domain.Config, logger domain.Logger) domain.CheckResult {
	if cfg == nil {
		return domain.CheckResult{
			Name:    "Linear MCP",
			Status:  domain.CheckSkip,
			Message: "skipped (config not available)",
		}
	}

	prompt := fmt.Sprintf("Reply with only the word OK. If you have access to the Linear MCP server for team %q, reply OK.", cfg.Tracker.Team)
	_, err := RunClaudeOnce(ctx, cfg, prompt, io.Discard, logger, WithAllowedTools(LinearMCPAllowedTools...))
	if err != nil {
		return domain.CheckResult{
			Name:    "Linear MCP",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("claude execution failed: %v", err),
			Hint: "run \"claude mcp add --transport http --scope project linear https://mcp.linear.app/mcp\" in your project root\n" +
				"  (a fully compatible local-only Linear MCP alternative is planned — check the project README for updates)",
		}
	}

	return domain.CheckResult{
		Name:    "Linear MCP",
		Status:  domain.CheckOK,
		Message: fmt.Sprintf("claude responded (team: %s)", cfg.Tracker.Team),
	}
}

// CheckStateDir verifies that the .siren/ state directory exists or can be
// created, and that it is writable. Uses a temporary file probe to confirm.
func CheckStateDir(baseDir string) domain.CheckResult {
	dir := filepath.Join(baseDir, domain.StateDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return domain.CheckResult{
			Name:    "State Dir",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("cannot create %s: %v", dir, err),
			Hint:    `check directory permissions or run "sightjack init"`,
		}
	}
	probe := filepath.Join(dir, ".doctor_probe")
	if err := os.WriteFile(probe, []byte("ok"), 0644); err != nil {
		return domain.CheckResult{
			Name:    "State Dir",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("%s is not writable: %v", dir, err),
			Hint:    "check file permissions on the .siren/ directory",
		}
	}
	_ = os.Remove(probe)
	return domain.CheckResult{
		Name:    "State Dir",
		Status:  domain.CheckOK,
		Message: fmt.Sprintf("%s writable", dir),
	}
}

// CheckSkills verifies that SKILL.md files exist under .siren/skills/
// and that their frontmatter contains a dmail-schema-version field.
func CheckSkills(baseDir string) domain.CheckResult {
	skillNames := []string{"dmail-sendable", "dmail-readable"}
	skillsDir := filepath.Join(baseDir, domain.StateDir, "skills")

	for _, name := range skillNames {
		path := filepath.Join(skillsDir, name, "SKILL.md")
		data, err := os.ReadFile(path)
		if err != nil {
			return domain.CheckResult{
				Name:    "Skills",
				Status:  domain.CheckFail,
				Message: fmt.Sprintf("%s/SKILL.md: %v", name, err),
				Hint:    `run "sightjack init" to regenerate skill files`,
			}
		}
		content := string(data)
		if !strings.Contains(content, "dmail-schema-version:") {
			return domain.CheckResult{
				Name:    "Skills",
				Status:  domain.CheckFail,
				Message: fmt.Sprintf("%s/SKILL.md: missing dmail-schema-version", name),
				Hint:    `run "sightjack init" to regenerate skill files`,
			}
		}
	}

	// Check for deprecated "kind: feedback" (split into design-feedback / implementation-feedback)
	for _, name := range skillNames {
		path := filepath.Join(skillsDir, name, "SKILL.md")
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		content := string(data)
		if strings.Contains(content, "kind: feedback") &&
			!strings.Contains(content, "kind: design-feedback") &&
			!strings.Contains(content, "kind: implementation-feedback") {
			return domain.CheckResult{
				Name:    "Skills",
				Status:  domain.CheckFail,
				Message: fmt.Sprintf("%s/SKILL.md uses deprecated kind 'feedback'", name),
				Hint:    `run "sightjack init --force" to regenerate skills with updated kind (feedback → design-feedback)`,
			}
		}
	}

	return domain.CheckResult{
		Name:    "Skills",
		Status:  domain.CheckOK,
		Message: fmt.Sprintf("%d skill(s) validated", len(skillNames)),
	}
}

// CheckEventStore validates the event store directory structure and JSONL integrity.
// Delegates to eventsource.ValidateStore for the actual file-level checks.
// Returns CheckSkip if the events directory does not exist yet.
func CheckEventStore(baseDir string) domain.CheckResult {
	stateDir := filepath.Join(baseDir, domain.StateDir)
	health := eventsource.ValidateStore(stateDir)

	if health.NotFound {
		return domain.CheckResult{
			Name:    "Event Store",
			Status:  domain.CheckSkip,
			Message: "events/ not found",
		}
	}

	if health.Err != nil {
		return domain.CheckResult{
			Name:    "Event Store",
			Status:  domain.CheckFail,
			Message: health.Err.Error(),
			Hint:    health.ErrHint,
		}
	}

	if health.Sessions == 0 {
		return domain.CheckResult{
			Name:    "Event Store",
			Status:  domain.CheckOK,
			Message: "no event files yet",
		}
	}

	return domain.CheckResult{
		Name:    "Event Store",
		Status:  domain.CheckOK,
		Message: fmt.Sprintf("%d session(s), %d event(s) OK", health.Sessions, health.Events),
	}
}

// RunDoctor executes all health checks and returns the results.
// The configPath is loaded to obtain tool configuration; if loading fails
// the config check reports failure but other checks continue where possible.
// baseDir is used to verify the .siren/ state directory is writable.
func RunDoctor(ctx context.Context, configPath string, baseDir string, logger domain.Logger) []domain.CheckResult {
	if logger == nil {
		logger = &domain.NopLogger{}
	}
	var results []domain.CheckResult

	// 1. Config check
	cfgResult := CheckConfig(configPath)
	results = append(results, cfgResult)

	// 2. State directory check
	results = append(results, CheckStateDir(baseDir))

	var cfg *domain.Config
	if cfgResult.Status == domain.CheckOK {
		// Re-load to use for subsequent checks (checkConfig already validated).
		cfg, _ = LoadConfig(configPath)
	}

	// 3. claude binary check
	claudeName := "claude"
	if cfg != nil && cfg.Assistant.Command != "" {
		claudeName = cfg.Assistant.Command
	}
	claudeResult := CheckTool(ctx, claudeName)
	results = append(results, claudeResult)

	// 4. git binary check
	results = append(results, CheckTool(ctx, "git"))

	// 5. Skills check
	results = append(results, CheckSkills(baseDir))

	// 5.5. Event store check
	results = append(results, CheckEventStore(baseDir))

	// 6. Claude Auth check (skip if claude binary unavailable)
	skipClaude := claudeResult.Status != domain.CheckOK
	if skipClaude {
		results = append(results, domain.CheckResult{
			Name:    "Claude Auth",
			Status:  domain.CheckSkip,
			Message: "skipped (claude not available)",
		})
	} else {
		authResult := CheckClaudeAuth(ctx, cfg, logger)
		results = append(results, authResult)
		if authResult.Status != domain.CheckOK {
			skipClaude = true
		}
	}

	// 7. Linear MCP connectivity (skip if claude binary or auth unavailable)
	if skipClaude {
		results = append(results, domain.CheckResult{
			Name:    "Linear MCP",
			Status:  domain.CheckSkip,
			Message: "skipped (claude not available)",
		})
	} else {
		results = append(results, CheckLinearMCP(ctx, cfg, logger))
	}

	// 8. Success rate (informational, never fails)
	allEvents, evErr := LoadAllEvents(ctx, baseDir)
	if evErr != nil || len(allEvents) == 0 {
		results = append(results, domain.CheckResult{
			Name:    "success-rate",
			Status:  domain.CheckOK,
			Message: "no events",
		})
	} else {
		rate := domain.SuccessRate(allEvents)
		var success, total int
		for _, ev := range allEvents {
			switch ev.Type {
			case domain.EventWaveApplied:
				success++
				total++
			case domain.EventWaveRejected:
				total++
			}
		}
		results = append(results, domain.CheckResult{
			Name:    "success-rate",
			Status:  domain.CheckOK,
			Message: domain.FormatSuccessRate(rate, success, total),
		})
	}

	return results
}
