package sightjack

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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
func checkLinearMCP(ctx context.Context, cfg *Config, logger *Logger) CheckResult {
	if cfg == nil {
		return CheckResult{
			Name:    "Linear MCP",
			Status:  CheckSkip,
			Message: "skipped (config not available)",
		}
	}

	prompt := fmt.Sprintf("Reply with only the word OK. If you have access to the Linear MCP server for team %q, reply OK.", cfg.Linear.Team)
	_, err := RunClaudeOnce(ctx, cfg, prompt, io.Discard, logger)
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

// checkStateDir verifies that the .siren/ state directory exists or can be
// created, and that it is writable. Uses a temporary file probe to confirm.
func checkStateDir(baseDir string) CheckResult {
	dir := filepath.Join(baseDir, stateDir)
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
	os.Remove(probe)
	return CheckResult{
		Name:    "State Dir",
		Status:  CheckOK,
		Message: fmt.Sprintf("%s writable", dir),
	}
}

// checkSkills verifies that SKILL.md files exist under .siren/skills/
// and that their frontmatter contains a dmail-schema-version field.
func checkSkills(baseDir string) CheckResult {
	skillNames := []string{"dmail-sendable", "dmail-readable"}
	skillsDir := filepath.Join(baseDir, stateDir, "skills")

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
func RunDoctor(ctx context.Context, configPath string, baseDir string, logger *Logger) []CheckResult {
	if logger == nil {
		logger = NewLogger(nil, false)
	}
	var results []CheckResult

	// 1. Config check
	cfgResult := checkConfig(configPath)
	results = append(results, cfgResult)

	// 2. State directory check
	results = append(results, checkStateDir(baseDir))

	var cfg *Config
	if cfgResult.Status == CheckOK {
		// Re-load to use for subsequent checks (checkConfig already validated).
		cfg, _ = LoadConfig(configPath)
	}

	// 3. claude binary check
	claudeName := "claude"
	if cfg != nil && cfg.Claude.Command != "" {
		claudeName = cfg.Claude.Command
	}
	claudeResult := checkTool(ctx, claudeName)
	results = append(results, claudeResult)

	// 4. git binary check
	results = append(results, checkTool(ctx, "git"))

	// 5. Skills check
	results = append(results, checkSkills(baseDir))

	// 6. Linear MCP connectivity (skip if claude unavailable)
	if claudeResult.Status != CheckOK {
		results = append(results, CheckResult{
			Name:    "Linear MCP",
			Status:  CheckSkip,
			Message: "skipped (claude not available)",
		})
	} else {
		results = append(results, checkLinearMCP(ctx, cfg, logger))
	}

	return results
}
