package session

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/eventsource"
	"github.com/hironow/sightjack/internal/platform"
)

// CheckConfig validates that the config file exists and can be loaded.
func CheckConfig(configPath string) domain.DoctorCheck {
	_, err := LoadConfig(configPath)
	if err != nil {
		return domain.DoctorCheck{
			Name:    "Config",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("%s: %v", configPath, err),
			Hint:    `run "sightjack init --team <TEAM> --project <PROJECT>" to create a config file`,
		}
	}
	return domain.DoctorCheck{
		Name:    "Config",
		Status:  domain.CheckOK,
		Message: fmt.Sprintf("%s loaded successfully", configPath),
	}
}

// CheckTool verifies that a CLI tool is installed and executable.
// It runs `<tool> --version` to confirm functionality.
func CheckTool(ctx context.Context, name string) domain.DoctorCheck {
	path, err := lookPath(name)
	if err != nil {
		return domain.DoctorCheck{
			Name:    name,
			Status:  domain.CheckFail,
			Message: "command not found",
			Hint:    fmt.Sprintf("install %s and ensure it is in PATH", name),
		}
	}

	out, err := newCmd(ctx, name, "--version").Output()
	if err != nil {
		return domain.DoctorCheck{
			Name:    name,
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("found at %s but --version failed: %v", path, err),
			Hint:    fmt.Sprintf("%s may be corrupted; reinstall it", name),
		}
	}

	version := strings.TrimSpace(strings.Split(string(out), "\n")[0])
	return domain.DoctorCheck{
		Name:    name,
		Status:  domain.CheckOK,
		Message: fmt.Sprintf("%s (%s)", path, version),
	}
}

// checkClaudeAuth determines if the Claude CLI is authenticated by
// interpreting the result of running `claude mcp list`. A successful
// command execution (no error) indicates the CLI is authenticated.
func checkClaudeAuth(mcpOutput string, mcpErr error) domain.DoctorCheck {
	if mcpErr != nil {
		return domain.DoctorCheck{
			Name:    "Claude Auth",
			Status:  domain.CheckFail,
			Message: "not authenticated: " + mcpErr.Error(),
			Hint:    `run "claude login" to authenticate (in Docker: set CLAUDE_CONFIG_DIR=~/.claude to use host credentials)`,
		}
	}
	return domain.DoctorCheck{
		Name:    "Claude Auth",
		Status:  domain.CheckOK,
		Message: "authenticated",
	}
}

// checkLinearMCP parses `claude mcp list` output for Linear MCP connection.
// Looks for a line containing "linear", "✓", and "connected" (case-insensitive).
// Requires "✓" to avoid false positives from "disconnected" or "not connected".
func checkLinearMCP(mcpOutput string, mcpErr error) domain.DoctorCheck {
	if mcpErr != nil {
		return domain.DoctorCheck{
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
			return domain.DoctorCheck{
				Name:    "Linear MCP",
				Status:  domain.CheckOK,
				Message: "Linear MCP connected",
			}
		}
	}

	return domain.DoctorCheck{
		Name:    "Linear MCP",
		Status:  domain.CheckFail,
		Message: "Linear MCP not found or not connected",
		Hint: "run \"claude mcp add --transport http --scope project linear https://mcp.linear.app/mcp\" in your project root\n" +
			"  (a fully compatible local-only Linear MCP alternative is planned — check the project README for updates)",
	}
}

// checkClaudeInference determines if the Claude CLI can perform inference
// by interpreting the result of a minimal "1+1=" prompt.
func checkClaudeInference(output string, err error) domain.DoctorCheck {
	if err != nil {
		return domain.DoctorCheck{
			Name:    "claude-inference",
			Status:  domain.CheckFail,
			Message: "inference failed: " + err.Error(),
			Hint: `"signal: killed" = CLI startup too slow (timeout 60s); ` +
				`"nested session" = CLAUDECODE env var leaked (doctor should filter it); ` +
				`otherwise check API key, quota, and model access`,
		}
	}
	if strings.TrimSpace(output) != "2" {
		return domain.DoctorCheck{
			Name:    "claude-inference",
			Status:  domain.CheckFail,
			Message: "unexpected response: " + strings.TrimSpace(output),
			Hint:    "model returned unexpected output; check model access and API quota",
		}
	}
	return domain.DoctorCheck{
		Name:    "claude-inference",
		Status:  domain.CheckOK,
		Message: "inference OK",
	}
}

// CheckStateDir verifies that the .siren/ state directory exists or can be
// created, and that it is writable. Uses a temporary file probe to confirm.
func CheckStateDir(baseDir string) domain.DoctorCheck {
	dir := filepath.Join(baseDir, domain.StateDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return domain.DoctorCheck{
			Name:    "State Dir",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("cannot create %s: %v", dir, err),
			Hint:    `check directory permissions or run "sightjack init"`,
		}
	}
	probe := filepath.Join(dir, ".doctor_probe")
	if err := os.WriteFile(probe, []byte("ok"), 0644); err != nil {
		return domain.DoctorCheck{
			Name:    "State Dir",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("%s is not writable: %v", dir, err),
			Hint:    "check file permissions on the .siren/ directory",
		}
	}
	_ = os.Remove(probe)
	return domain.DoctorCheck{
		Name:    "State Dir",
		Status:  domain.CheckOK,
		Message: fmt.Sprintf("%s writable", dir),
	}
}

// CheckSkills verifies that SKILL.md files exist under .siren/skills/
// and that their frontmatter contains a dmail-schema-version field.
func CheckSkills(baseDir string) domain.DoctorCheck {
	skillNames := []string{"dmail-sendable", "dmail-readable"}
	skillsDir := filepath.Join(baseDir, domain.StateDir, "skills")

	for _, name := range skillNames {
		path := filepath.Join(skillsDir, name, "SKILL.md")
		data, err := os.ReadFile(path)
		if err != nil {
			return domain.DoctorCheck{
				Name:    "Skills",
				Status:  domain.CheckFail,
				Message: fmt.Sprintf("%s/SKILL.md: %v", name, err),
				Hint:    `run "sightjack init" to regenerate skill files`,
			}
		}
		content := string(data)
		if !strings.Contains(content, "dmail-schema-version:") {
			return domain.DoctorCheck{
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
			return domain.DoctorCheck{
				Name:    "Skills",
				Status:  domain.CheckFail,
				Message: fmt.Sprintf("%s/SKILL.md uses deprecated kind 'feedback'", name),
				Hint:    "deprecated kind 'feedback'; migrate to 'design-feedback' (run 'sightjack init --force' to regenerate SKILL.md)",
			}
		}
	}

	return domain.DoctorCheck{
		Name:    "Skills",
		Status:  domain.CheckOK,
		Message: fmt.Sprintf("%d skill(s) validated", len(skillNames)),
	}
}

// CheckEventStore validates the event store directory structure and JSONL integrity.
// Delegates to eventsource.ValidateStore for the actual file-level checks.
// Returns CheckSkip if the events directory does not exist yet.
func CheckEventStore(baseDir string) domain.DoctorCheck {
	stateDir := filepath.Join(baseDir, domain.StateDir)
	health := eventsource.ValidateStore(stateDir)

	if health.NotFound {
		return domain.DoctorCheck{
			Name:    "Event Store",
			Status:  domain.CheckSkip,
			Message: "events/ not found",
		}
	}

	if health.Err != nil {
		return domain.DoctorCheck{
			Name:    "Event Store",
			Status:  domain.CheckFail,
			Message: health.Err.Error(),
			Hint:    health.ErrHint,
		}
	}

	if health.Sessions == 0 {
		return domain.DoctorCheck{
			Name:    "Event Store",
			Status:  domain.CheckOK,
			Message: "no event files yet",
		}
	}

	return domain.DoctorCheck{
		Name:    "Event Store",
		Status:  domain.CheckOK,
		Message: fmt.Sprintf("%d session(s), %d event(s) OK", health.Sessions, health.Events),
	}
}

// RunDoctor executes all health checks and returns the results.
// The configPath is loaded to obtain tool configuration; if loading fails
// the config check reports failure but other checks continue where possible.
// baseDir is used to verify the .siren/ state directory is writable.
func RunDoctor(ctx context.Context, configPath string, baseDir string, logger domain.Logger) []domain.DoctorCheck {
	if logger == nil {
		logger = &domain.NopLogger{}
	}
	var results []domain.DoctorCheck

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
		results = append(results, domain.DoctorCheck{
			Name:    "Claude Auth",
			Status:  domain.CheckSkip,
			Message: "skipped (config not loaded)",
		})
		results = append(results, domain.DoctorCheck{
			Name:    "Linear MCP",
			Status:  domain.CheckSkip,
			Message: "skipped (config not loaded)",
		})
		results = append(results, domain.DoctorCheck{
			Name:    "claude-inference",
			Status:  domain.CheckSkip,
			Message: "skipped (config not loaded)",
		})
		results = append(results, domain.DoctorCheck{
			Name:    "context-budget",
			Status:  domain.CheckSkip,
			Message: "skipped (config not loaded)",
		})
	} else if claudeResult.Status != domain.CheckOK {
		results = append(results, domain.DoctorCheck{
			Name:    "Claude Auth",
			Status:  domain.CheckSkip,
			Message: "skipped (claude not available)",
		})
		results = append(results, domain.DoctorCheck{
			Name:    "Linear MCP",
			Status:  domain.CheckSkip,
			Message: "skipped (claude not available)",
		})
		results = append(results, domain.DoctorCheck{
			Name:    "claude-inference",
			Status:  domain.CheckSkip,
			Message: "skipped (claude not available)",
		})
		results = append(results, domain.DoctorCheck{
			Name:    "context-budget",
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
			results = append(results, domain.DoctorCheck{
				Name:    "Linear MCP",
				Status:  domain.CheckSkip,
				Message: "skipped (claude not authenticated)",
			})
			results = append(results, domain.DoctorCheck{
				Name:    "claude-inference",
				Status:  domain.CheckSkip,
				Message: "skipped (auth failed)",
			})
			results = append(results, domain.DoctorCheck{
				Name:    "context-budget",
				Status:  domain.CheckSkip,
				Message: "skipped (auth failed)",
			})
		} else {
			results = append(results, checkLinearMCP(mcpOutput, mcpErr))

			inferCtx, inferCancel := context.WithTimeout(ctx, 60*time.Second)
			inferCmd := newCmd(inferCtx, claudeName, "--print", "--output-format", "stream-json", "--max-turns", "1", "1+1=")
			inferCmd.Env = filterEnv(os.Environ(), "CLAUDECODE")
			inferOut, inferErr := inferCmd.Output()
			inferCancel()
			inferOutput := string(inferOut)
			results = append(results, checkClaudeInference(strings.TrimSpace(ExtractStreamResult(inferOutput)), inferErr))

			// Context budget check: reuse inference stream-json output
			results = append(results, CheckContextBudget(inferOutput, baseDir))
		}
	}

	// --- Metrics ---
	allEvents, evErr := LoadAllEvents(ctx, baseDir)
	if evErr != nil || len(allEvents) == 0 {
		results = append(results, domain.DoctorCheck{
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
		results = append(results, domain.DoctorCheck{
			Name:    "success-rate",
			Status:  domain.CheckOK,
			Message: domain.FormatSuccessRate(rate, success, total),
		})
	}

	return results
}

// ExtractStreamResult parses stream-json output and returns the "result" field
// from the result message. Used to reuse inference check output for inference validation.
func ExtractStreamResult(streamJSON string) string {
	for _, line := range strings.Split(streamJSON, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var msg struct {
			Type   string `json:"type"`
			Result string `json:"result"`
		}
		if err := json.Unmarshal([]byte(line), &msg); err == nil && msg.Type == "result" {
			return msg.Result
		}
	}
	return ""
}

// CheckContextBudget parses stream-json output from a Claude CLI invocation
// and estimates context consumption by category.
func CheckContextBudget(streamJSON string, baseDir string) domain.DoctorCheck {
	var messages []*platform.StreamMessage
	for _, line := range strings.Split(streamJSON, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		msg, err := platform.ParseStreamMessage([]byte(line))
		if err != nil {
			continue
		}
		messages = append(messages, msg)
	}

	report := platform.CalculateContextBudget(messages)
	breakdown := report.DetailedBreakdown()

	// Build detailed message lines
	var lines []string
	for _, item := range breakdown {
		marker := ""
		if item.Heaviest {
			marker = " <- heaviest"
		}
		if item.Category == "hooks" {
			if item.Bytes > 0 {
				lines = append(lines, fmt.Sprintf("  hooks: %d bytes (%d tok)%s", item.Bytes, item.Tokens, marker))
			}
		} else {
			if item.Count > 0 {
				lines = append(lines, fmt.Sprintf("  %s: %d (%d tok)%s", item.Category, item.Count, item.Tokens, marker))
			}
		}
	}

	status := domain.CheckOK
	msg := fmt.Sprintf("estimated %d tokens", report.EstimatedTokens)
	if report.Exceeds(platform.DefaultContextBudgetThreshold) {
		status = domain.CheckWarn
		msg = fmt.Sprintf("estimated %d tokens (threshold: %d)", report.EstimatedTokens, platform.DefaultContextBudgetThreshold)
	}
	if len(lines) > 0 {
		msg += "\n" + strings.Join(lines, "\n")
	}

	result := domain.DoctorCheck{
		Name:    "context-budget",
		Status:  status,
		Message: msg,
	}

	// Hint logic: only when threshold exceeded
	if report.Exceeds(platform.DefaultContextBudgetThreshold) {
		projectSettings := filepath.Join(baseDir, ".claude", "settings.json")
		if _, err := os.Stat(projectSettings); err == nil {
			result.Hint = ".claude/settings.json の設定を見直してください"
		} else {
			var heaviest string
			for _, item := range breakdown {
				if item.Heaviest {
					heaviest = item.Category
					break
				}
			}
			switch heaviest {
			case "mcp_servers":
				result.Hint = ".claude/settings.json をプロジェクトに作成し、必要な MCP server のみ定義を推奨"
			case "tools":
				result.Hint = "tools は plugins/MCP 由来 → .claude/settings.json で plugins/MCP を絞ることを推奨"
			default:
				result.Hint = ".claude/settings.json をプロジェクトに作成し、必要なプラグインのみ有効化を推奨"
			}
		}
	}

	return result
}

// filterEnv returns a copy of env with the named variable removed.
// Used to unset CLAUDECODE so that doctor's inference check does not
// trigger the nested-session guard in Claude Code.
func filterEnv(env []string, name string) []string {
	prefix := name + "="
	out := make([]string, 0, len(env))
	for _, e := range env {
		if !strings.HasPrefix(e, prefix) {
			out = append(out, e)
		}
	}
	return out
}
