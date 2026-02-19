package sightjack

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
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

// checkConfig validates that the config file exists and can be loaded.
func checkConfig(configPath string) CheckResult {
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

// checkTool verifies that a CLI tool is installed and executable.
// It runs `<tool> --version` to confirm functionality.
func checkTool(ctx context.Context, name string) CheckResult {
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

// checkLinearMCP verifies Linear MCP connectivity by sending a lightweight
// prompt to Claude and checking if it responds without error.
// Returns CheckSkip if cfg is nil (config loading failed).
func checkLinearMCP(ctx context.Context, cfg *Config) CheckResult {
	if cfg == nil {
		return CheckResult{
			Name:    "Linear MCP",
			Status:  CheckSkip,
			Message: "skipped (config not available)",
		}
	}

	prompt := fmt.Sprintf("Reply with only the word OK. If you have access to the Linear MCP server for team %q, reply OK.", cfg.Linear.Team)
	_, err := RunClaudeOnce(ctx, cfg, prompt, io.Discard)
	if err != nil {
		return CheckResult{
			Name:    "Linear MCP",
			Status:  CheckFail,
			Message: fmt.Sprintf("claude execution failed: %v", err),
		}
	}

	return CheckResult{
		Name:    "Linear MCP",
		Status:  CheckOK,
		Message: fmt.Sprintf("claude responded (team: %s)", cfg.Linear.Team),
	}
}

// RunDoctor executes all health checks and returns the results.
// The configPath is loaded to obtain tool configuration; if loading fails
// the config check reports failure but other checks continue where possible.
func RunDoctor(ctx context.Context, configPath string) []CheckResult {
	var results []CheckResult

	// 1. Config check
	cfgResult := checkConfig(configPath)
	results = append(results, cfgResult)

	var cfg *Config
	if cfgResult.Status == CheckOK {
		// Re-load to use for subsequent checks (checkConfig already validated).
		cfg, _ = LoadConfig(configPath)
	}

	// 2. claude binary check
	claudeName := "claude"
	if cfg != nil && cfg.Claude.Command != "" {
		claudeName = cfg.Claude.Command
	}
	claudeResult := checkTool(ctx, claudeName)
	results = append(results, claudeResult)

	// 3. git binary check
	results = append(results, checkTool(ctx, "git"))

	// 4. Linear MCP connectivity (skip if claude unavailable)
	if claudeResult.Status != CheckOK {
		results = append(results, CheckResult{
			Name:    "Linear MCP",
			Status:  CheckSkip,
			Message: "skipped (claude not available)",
		})
	} else {
		results = append(results, checkLinearMCP(ctx, cfg))
	}

	return results
}
