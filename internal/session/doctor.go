package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	path, err := lookPath(name)
	if err != nil {
		return domain.CheckResult{
			Name:    name,
			Status:  domain.CheckFail,
			Message: "command not found",
			Hint:    fmt.Sprintf("install %s and ensure it is in PATH", name),
		}
	}

	out, err := newCmd(ctx, name, "--version").Output()
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

// checkClaudeAuth determines if the Claude CLI is authenticated by
// interpreting the result of running `claude mcp list`. A successful
// command execution (no error) indicates the CLI is authenticated.
func checkClaudeAuth(mcpOutput string, mcpErr error) domain.CheckResult {
	if mcpErr != nil {
		return domain.CheckResult{
			Name:    "Claude Auth",
			Status:  domain.CheckFail,
			Message: "not authenticated: " + mcpErr.Error(),
			Hint:    `run "claude login" to authenticate (in Docker: set CLAUDE_CONFIG_DIR=~/.claude to use host credentials)`,
		}
	}
	return domain.CheckResult{
		Name:    "Claude Auth",
		Status:  domain.CheckOK,
		Message: "authenticated",
	}
}

// checkLinearMCP parses `claude mcp list` output for Linear MCP connection.
// Looks for a line containing "linear", "✓", and "connected" (case-insensitive).
// Requires "✓" to avoid false positives from "disconnected" or "not connected".
func checkLinearMCP(mcpOutput string, mcpErr error) domain.CheckResult {
	if mcpErr != nil {
		return domain.CheckResult{
			Name:    "Linear MCP",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("claude mcp list failed: %v", mcpErr),
			Hint:    `ensure Claude CLI is authenticated with "claude login"`,
		}
	}

	output := strings.ToLower(mcpOutput)
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "linear") &&
			strings.Contains(line, "✓") &&
			strings.Contains(line, "connected") {
			return domain.CheckResult{
				Name:    "Linear MCP",
				Status:  domain.CheckOK,
				Message: "Linear MCP connected",
			}
		}
	}

	return domain.CheckResult{
		Name:    "Linear MCP",
		Status:  domain.CheckFail,
		Message: "Linear MCP not found or not connected",
		Hint: "run \"claude mcp add --transport http --scope project linear https://mcp.linear.app/mcp\" in your project root\n" +
			"  (a fully compatible local-only Linear MCP alternative is planned — check the project README for updates)",
	}
}

// checkClaudeInference determines if the Claude CLI can perform inference
// by interpreting the result of a minimal "1+1=" prompt.
func checkClaudeInference(output string, err error) domain.CheckResult {
	if err != nil {
		return domain.CheckResult{
			Name:    "claude-inference",
			Status:  domain.CheckFail,
			Message: "inference failed: " + err.Error(),
			Hint:    "check API key, quota, and model access",
		}
	}
	if !strings.Contains(output, "2") {
		return domain.CheckResult{
			Name:    "claude-inference",
			Status:  domain.CheckFail,
			Message: "unexpected response",
			Hint:    "check API key, quota, and model access",
		}
	}
	return domain.CheckResult{
		Name:    "claude-inference",
		Status:  domain.CheckOK,
		Message: "inference OK",
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

	// --- Binaries ---
	results = append(results, CheckTool(ctx, "git"))

	// Load config early to get claudeCmd
	var cfg *domain.Config
	cfgResult := CheckConfig(configPath)
	if cfgResult.Status == domain.CheckOK {
		cfg, _ = LoadConfig(configPath)
	}

	claudeName := domain.DefaultClaudeCmd
	if cfg != nil && cfg.ClaudeCmd != "" {
		claudeName = cfg.ClaudeCmd
	}
	claudeResult := CheckTool(ctx, claudeName)
	results = append(results, claudeResult)

	// --- State ---
	results = append(results, CheckStateDir(baseDir))
	results = append(results, cfgResult)

	// --- Data ---
	results = append(results, CheckSkills(baseDir))
	results = append(results, CheckEventStore(baseDir))

	// --- Connectivity ---
	if cfgResult.Status != domain.CheckOK {
		results = append(results, domain.CheckResult{
			Name:    "Claude Auth",
			Status:  domain.CheckSkip,
			Message: "skipped (config not loaded)",
		})
		results = append(results, domain.CheckResult{
			Name:    "Linear MCP",
			Status:  domain.CheckSkip,
			Message: "skipped (config not loaded)",
		})
		results = append(results, domain.CheckResult{
			Name:    "claude-inference",
			Status:  domain.CheckSkip,
			Message: "skipped (config not loaded)",
		})
	} else if claudeResult.Status != domain.CheckOK {
		results = append(results, domain.CheckResult{
			Name:    "Claude Auth",
			Status:  domain.CheckSkip,
			Message: "skipped (claude not available)",
		})
		results = append(results, domain.CheckResult{
			Name:    "Linear MCP",
			Status:  domain.CheckSkip,
			Message: "skipped (claude not available)",
		})
		results = append(results, domain.CheckResult{
			Name:    "claude-inference",
			Status:  domain.CheckSkip,
			Message: "skipped (claude not available)",
		})
	} else {
		mcpCtx, mcpCancel := context.WithTimeout(ctx, 10*time.Second)
		cmd := newCmd(mcpCtx, claudeName, "mcp", "list")
		out, mcpErr := cmd.Output()
		mcpCancel()
		mcpOutput := string(out)

		authResult := checkClaudeAuth(mcpOutput, mcpErr)
		results = append(results, authResult)

		if authResult.Status != domain.CheckOK {
			results = append(results, domain.CheckResult{
				Name:    "Linear MCP",
				Status:  domain.CheckSkip,
				Message: "skipped (claude not authenticated)",
			})
			results = append(results, domain.CheckResult{
				Name:    "claude-inference",
				Status:  domain.CheckSkip,
				Message: "skipped (auth failed)",
			})
		} else {
			results = append(results, checkLinearMCP(mcpOutput, mcpErr))

			inferCtx, inferCancel := context.WithTimeout(ctx, 15*time.Second)
			inferCmd := newCmd(inferCtx, claudeName, "--print", "--output-format", "text", "--max-turns", "1", "1+1=")
			inferOut, inferErr := inferCmd.Output()
			inferCancel()
			results = append(results, checkClaudeInference(string(inferOut), inferErr))
		}
	}

	// --- Metrics ---
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
