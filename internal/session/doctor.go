package session

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	sightjack "github.com/hironow/sightjack"
	"github.com/hironow/sightjack/internal/domain"
)

// CheckStatus represents the outcome of a single doctor check.
type CheckStatus int

const (
	CheckOK CheckStatus = iota
	CheckFail
	CheckSkip
)

// CheckResult holds the outcome of a single doctor check.
type CheckResult struct {
	Name    string
	Status  CheckStatus
	Message string
	Hint    string // optional remediation hint shown on failure
}

// StatusLabel returns a display string for the check status.
func (s CheckStatus) StatusLabel() string {
	switch s {
	case CheckOK:
		return "OK"
	case CheckFail:
		return "FAIL"
	case CheckSkip:
		return "SKIP"
	default:
		return "?"
	}
}

// CheckConfig validates that the config file exists and can be loaded.
func CheckConfig(configPath string) CheckResult {
	_, err := LoadConfig(configPath)
	if err != nil {
		return CheckResult{
			Name:    "Config",
			Status:  CheckFail,
			Message: fmt.Sprintf("%s: %v", configPath, err),
		}
	}
	return CheckResult{
		Name:    "Config",
		Status:  CheckOK,
		Message: fmt.Sprintf("%s loaded successfully", configPath),
	}
}

// CheckTool verifies that a CLI tool is installed and executable.
// It runs `<tool> --version` to confirm functionality.
func CheckTool(ctx context.Context, name string) CheckResult {
	path, err := exec.LookPath(name)
	if err != nil {
		return CheckResult{
			Name:    name,
			Status:  CheckFail,
			Message: "command not found",
		}
	}

	out, err := exec.CommandContext(ctx, path, "--version").Output()
	if err != nil {
		return CheckResult{
			Name:    name,
			Status:  CheckFail,
			Message: fmt.Sprintf("found at %s but --version failed: %v", path, err),
		}
	}

	version := strings.TrimSpace(strings.Split(string(out), "\n")[0])
	return CheckResult{
		Name:    name,
		Status:  CheckOK,
		Message: fmt.Sprintf("%s (%s)", path, version),
	}
}

// CheckClaudeAuth verifies that Claude Code is authenticated by sending a
// simple prompt that does not require any MCP server.
// Returns CheckSkip if cfg is nil (config loading failed).
func CheckClaudeAuth(ctx context.Context, cfg *sightjack.Config, logger *domain.Logger) CheckResult {
	if cfg == nil {
		return CheckResult{
			Name:    "Claude Auth",
			Status:  CheckSkip,
			Message: "skipped (config not available)",
		}
	}

	output, err := RunClaudeOnce(ctx, cfg, "Reply with only the word OK.", io.Discard, logger, WithAllowedTools("Write"))
	if err != nil {
		hint := fmt.Sprintf("claude execution failed: %v", err)
		if strings.Contains(output, "Not logged in") {
			return CheckResult{
				Name:    "Claude Auth",
				Status:  CheckFail,
				Message: "not logged in",
				Hint:    `run "claude login" then "/login" inside the session`,
			}
		}
		return CheckResult{
			Name:    "Claude Auth",
			Status:  CheckFail,
			Message: hint,
		}
	}

	return CheckResult{
		Name:    "Claude Auth",
		Status:  CheckOK,
		Message: "authenticated",
	}
}

// CheckLinearMCP verifies Linear MCP connectivity by sending a prompt that
// references the configured Linear team.
// Returns CheckSkip if cfg is nil (config loading failed).
func CheckLinearMCP(ctx context.Context, cfg *sightjack.Config, logger *domain.Logger) CheckResult {
	if cfg == nil {
		return CheckResult{
			Name:    "Linear MCP",
			Status:  CheckSkip,
			Message: "skipped (config not available)",
		}
	}

	prompt := fmt.Sprintf("Reply with only the word OK. If you have access to the Linear MCP server for team %q, reply OK.", cfg.Linear.Team)
	_, err := RunClaudeOnce(ctx, cfg, prompt, io.Discard, logger, WithAllowedTools(LinearMCPAllowedTools...))
	if err != nil {
		return CheckResult{
			Name:    "Linear MCP",
			Status:  CheckFail,
			Message: fmt.Sprintf("claude execution failed: %v", err),
			Hint:    `run "claude mcp add --transport http --scope project linear https://mcp.linear.app/mcp" in your project root`,
		}
	}

	return CheckResult{
		Name:    "Linear MCP",
		Status:  CheckOK,
		Message: fmt.Sprintf("claude responded (team: %s)", cfg.Linear.Team),
	}
}

// CheckStateDir verifies that the .siren/ state directory exists or can be
// created, and that it is writable. Uses a temporary file probe to confirm.
func CheckStateDir(baseDir string) CheckResult {
	dir := filepath.Join(baseDir, sightjack.StateDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return CheckResult{
			Name:    "State Dir",
			Status:  CheckFail,
			Message: fmt.Sprintf("cannot create %s: %v", dir, err),
		}
	}
	probe := filepath.Join(dir, ".doctor_probe")
	if err := os.WriteFile(probe, []byte("ok"), 0644); err != nil {
		return CheckResult{
			Name:    "State Dir",
			Status:  CheckFail,
			Message: fmt.Sprintf("%s is not writable: %v", dir, err),
		}
	}
	_ = os.Remove(probe)
	return CheckResult{
		Name:    "State Dir",
		Status:  CheckOK,
		Message: fmt.Sprintf("%s writable", dir),
	}
}

// CheckSkills verifies that SKILL.md files exist under .siren/skills/
// and that their frontmatter contains a dmail-schema-version field.
func CheckSkills(baseDir string) CheckResult {
	skillNames := []string{"dmail-sendable", "dmail-readable"}
	skillsDir := filepath.Join(baseDir, sightjack.StateDir, "skills")

	for _, name := range skillNames {
		path := filepath.Join(skillsDir, name, "SKILL.md")
		data, err := os.ReadFile(path)
		if err != nil {
			return CheckResult{
				Name:    "Skills",
				Status:  CheckFail,
				Message: fmt.Sprintf("%s/SKILL.md: %v", name, err),
			}
		}
		content := string(data)
		if !strings.Contains(content, "dmail-schema-version:") {
			return CheckResult{
				Name:    "Skills",
				Status:  CheckFail,
				Message: fmt.Sprintf("%s/SKILL.md: missing dmail-schema-version", name),
			}
		}
	}

	return CheckResult{
		Name:    "Skills",
		Status:  CheckOK,
		Message: fmt.Sprintf("%d skill(s) validated", len(skillNames)),
	}
}

// RunDoctor executes all health checks and returns the results.
// The configPath is loaded to obtain tool configuration; if loading fails
// the config check reports failure but other checks continue where possible.
// baseDir is used to verify the .siren/ state directory is writable.
func RunDoctor(ctx context.Context, configPath string, baseDir string, logger *domain.Logger) []CheckResult {
	if logger == nil {
		logger = domain.NewLogger(nil, false)
	}
	var results []CheckResult

	// 1. Config check
	cfgResult := CheckConfig(configPath)
	results = append(results, cfgResult)

	// 2. State directory check
	results = append(results, CheckStateDir(baseDir))

	var cfg *sightjack.Config
	if cfgResult.Status == CheckOK {
		// Re-load to use for subsequent checks (checkConfig already validated).
		cfg, _ = LoadConfig(configPath)
	}

	// 3. claude binary check
	claudeName := "claude"
	if cfg != nil && cfg.Claude.Command != "" {
		claudeName = cfg.Claude.Command
	}
	claudeResult := CheckTool(ctx, claudeName)
	results = append(results, claudeResult)

	// 4. git binary check
	results = append(results, CheckTool(ctx, "git"))

	// 5. Skills check
	results = append(results, CheckSkills(baseDir))

	// 6. Claude Auth check (skip if claude binary unavailable)
	skipClaude := claudeResult.Status != CheckOK
	if skipClaude {
		results = append(results, CheckResult{
			Name:    "Claude Auth",
			Status:  CheckSkip,
			Message: "skipped (claude not available)",
		})
	} else {
		authResult := CheckClaudeAuth(ctx, cfg, logger)
		results = append(results, authResult)
		if authResult.Status != CheckOK {
			skipClaude = true
		}
	}

	// 7. Linear MCP connectivity (skip if claude binary or auth unavailable)
	if skipClaude {
		results = append(results, CheckResult{
			Name:    "Linear MCP",
			Status:  CheckSkip,
			Message: "skipped (claude not available)",
		})
	} else {
		results = append(results, CheckLinearMCP(ctx, cfg, logger))
	}

	// 8. Success rate (informational, never fails)
	allEvents, evErr := LoadAllEvents(baseDir)
	if evErr != nil || len(allEvents) == 0 {
		results = append(results, CheckResult{
			Name:    "success-rate",
			Status:  CheckOK,
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
		results = append(results, CheckResult{
			Name:    "success-rate",
			Status:  CheckOK,
			Message: domain.FormatSuccessRate(rate, success, total),
		})
	}

	return results
}
