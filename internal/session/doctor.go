package session

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/eventsource"
	"github.com/hironow/sightjack/internal/platform"
)

// installSkillsRefFn runs "uv tool install skills-ref". Injectable for testing.
var installSkillsRefFn = func() error {
	cmd := exec.Command("uv", "tool", "install", "skills-ref")
	return cmd.Run()
}

// findSkillsRefDirFn searches for skills-ref submodule directory relative to baseDir.
var findSkillsRefDirFn = findSkillsRefDir

// generateSkillsFn regenerates SKILL.md files. Injectable for testing.
var generateSkillsFn = func(baseDir string, logger domain.Logger) error {
	return InstallSkills(baseDir, platform.SkillsFS, logger)
}

// OverrideInstallSkillsRef replaces the skills-ref installer for testing.
func OverrideInstallSkillsRef(fn func() error) func() {
	old := installSkillsRefFn
	installSkillsRefFn = fn
	return func() { installSkillsRefFn = old }
}

// OverrideFindSkillsRefDir replaces the skills-ref directory finder for testing.
func OverrideFindSkillsRefDir(fn func(string) string) func() {
	old := findSkillsRefDirFn
	findSkillsRefDirFn = fn
	return func() { findSkillsRefDirFn = old }
}

// OverrideGenerateSkills replaces the skills generator for testing.
func OverrideGenerateSkills(fn func(string, domain.Logger) error) func() {
	old := generateSkillsFn
	generateSkillsFn = fn
	return func() { generateSkillsFn = old }
}

// findSkillsRefDir searches for skills-ref in common submodule locations
// relative to baseDir (the target repository root), not the current working directory.
func findSkillsRefDir(baseDir string) string {
	candidates := []string{
		filepath.Join(baseDir, "..", "skills-ref"),
		filepath.Join(baseDir, "..", "..", "skills-ref"),
	}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && fi.IsDir() {
			return c
		}
	}
	return ""
}

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

// CheckStateDir verifies that the .siren/ state directory exists and is
// writable. When repair is true and the directory is missing, it creates
// the directory and returns CheckFixed.
func CheckStateDir(baseDir string, repair bool) domain.DoctorCheck {
	dir := filepath.Join(baseDir, domain.StateDir)
	info, statErr := os.Stat(dir)
	if statErr != nil {
		if !repair {
			return domain.DoctorCheck{
				Name:    "State Dir",
				Status:  domain.CheckFail,
				Message: fmt.Sprintf("%s not found", dir),
				Hint:    `run "sightjack init" or "sightjack doctor --repair"`,
			}
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			return domain.DoctorCheck{
				Name:    "State Dir",
				Status:  domain.CheckFail,
				Message: fmt.Sprintf("cannot create %s: %v", dir, err),
				Hint:    `check directory permissions or run "sightjack init"`,
			}
		}
		return domain.DoctorCheck{
			Name:    "State Dir",
			Status:  domain.CheckFixed,
			Message: fmt.Sprintf("created %s", dir),
		}
	}
	if !info.IsDir() {
		return domain.DoctorCheck{
			Name:    "State Dir",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("%s exists but is not a directory", dir),
			Hint:    `remove the .siren file and run "sightjack init"`,
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

// CheckEventStore validates the event store directory structure, JSONL integrity,
// and corruption status. Delegates to eventsource.ValidateStore which uses the
// same json.Unmarshal judgment as the real event store replay.
// Returns CheckSkip if the events directory does not exist yet.
// checkDeadLetters reports outbox items that have exceeded max retry count.
func checkDeadLetters(baseDir string) domain.DoctorCheck {
	// Check DB file exists before opening (avoid creating dirs/DB as side effect)
	dbPath := filepath.Join(baseDir, domain.StateDir, ".run", "outbox.db")
	if _, err := os.Stat(dbPath); err != nil {
		return domain.DoctorCheck{
			Name:    "dead-letters",
			Status:  domain.CheckSkip,
			Message: "no outbox DB",
		}
	}
	store, err := NewOutboxStoreForDir(baseDir)
	if err != nil {
		return domain.DoctorCheck{
			Name:    "dead-letters",
			Status:  domain.CheckSkip,
			Message: "outbox store unavailable",
		}
	}
	defer store.Close()

	count, err := store.DeadLetterCount(context.Background())
	if err != nil {
		return domain.DoctorCheck{
			Name:    "dead-letters",
			Status:  domain.CheckSkip,
			Message: fmt.Sprintf("dead letter count: %v", err),
		}
	}
	if count > 0 {
		return domain.DoctorCheck{
			Name:    "dead-letters",
			Status:  domain.CheckWarn,
			Message: fmt.Sprintf("%d dead-lettered outbox item(s)", count),
			Hint:    "these items failed delivery 3+ times and are permanently stuck — inspect outbox.db in .siren/.run/",
		}
	}
	return domain.DoctorCheck{
		Name:    "dead-letters",
		Status:  domain.CheckOK,
		Message: "no dead-lettered items",
	}
}

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

	// Load errors (permission issues, etc.) — WARN level
	if len(health.LoadErrors) > 0 {
		msg := fmt.Sprintf("%d load error(s)", len(health.LoadErrors))
		if health.CorruptLines > 0 {
			msg = fmt.Sprintf("%d corrupt line(s), %d load error(s)", health.CorruptLines, len(health.LoadErrors))
		}
		return domain.DoctorCheck{
			Name:    "Event Store",
			Status:  domain.CheckWarn,
			Message: msg,
			Hint:    health.ErrHint,
		}
	}

	// Corrupt lines detected — WARN level (not FAIL: replay skips them)
	if health.CorruptLines > 0 {
		return domain.DoctorCheck{
			Name:    "Event Store",
			Status:  domain.CheckWarn,
			Message: fmt.Sprintf("%d session(s), %d event(s), %d corrupt line(s)", health.Sessions, health.Events, health.CorruptLines),
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
func RunDoctor(ctx context.Context, configPath string, baseDir string, logger domain.Logger, repair bool, mode domain.TrackingMode) []domain.DoctorCheck {
	if logger == nil {
		logger = &domain.NopLogger{}
	}
	var results []domain.DoctorCheck

	// --- Binaries ---
	results = append(results, CheckTool(ctx, "git"))
	if mode.IsWave() {
		ghCheck := CheckTool(ctx, "gh")
		results = append(results, ghCheck)
		if ghCheck.Status == domain.CheckOK {
			results = append(results, checkGHAuth(ctx))
		} else {
			results = append(results, domain.DoctorCheck{
				Name:    "gh-auth",
				Status:  domain.CheckSkip,
				Message: "skipped (gh not available)",
			})
		}
	}

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
	results = append(results, CheckStateDir(baseDir, repair))
	results = append(results, cfgResult)

	// --- Data ---
	skillResult := CheckSkills(baseDir)
	if repair && skillResult.Status == domain.CheckFail {
		if err := generateSkillsFn(baseDir, logger); err == nil {
			recheck := CheckSkills(baseDir)
			if recheck.Status == domain.CheckOK {
				results = append(results, domain.DoctorCheck{
					Name: "Skills", Status: domain.CheckFixed,
					Message: "regenerated SKILL.md files",
				})
			} else {
				results = append(results, skillResult)
			}
		} else {
			results = append(results, skillResult)
		}
	} else {
		results = append(results, skillResult)
	}
	results = append(results, CheckEventStore(baseDir))
	results = append(results, checkDeadLetters(baseDir))
	// --- Connectivity ---
	if cfgResult.Status != domain.CheckOK {
		results = append(results, domain.DoctorCheck{
			Name:    "claude-auth",
			Status:  domain.CheckSkip,
			Message: "skipped (config not loaded)",
		})
		results = append(results, domain.DoctorCheck{
			Name:    "linear-mcp",
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
			Name:    "claude-auth",
			Status:  domain.CheckSkip,
			Message: "skipped (claude not available)",
		})
		results = append(results, domain.DoctorCheck{
			Name:    "linear-mcp",
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

		authResult := checkClaudeAuth(mcpOutput, mcpErr, claudeName)
		results = append(results, authResult)
		// Linear MCP: skip in wave mode (no Linear dependency)
		if mode.IsWave() {
			results = append(results, domain.DoctorCheck{
				Name:    "linear-mcp",
				Status:  domain.CheckSkip,
				Message: "skipped (wave mode)",
			})
		} else if authResult.Status != domain.CheckOK {
			results = append(results, domain.DoctorCheck{
				Name:    "linear-mcp",
				Status:  domain.CheckSkip,
				Message: "skipped (auth failed)",
			})
		} else {
			results = append(results, checkLinearMCP(mcpOutput, mcpErr))
		}

		// Inference: runs independently of mcp list result.
		// MCP config issues don't affect core inference capability.
		inferCtx, inferCancel := context.WithTimeout(ctx, 3*time.Minute)
		inferCmd := newCmd(inferCtx, claudeName, "--print", "--verbose", "--output-format", "stream-json", "--max-turns", "1", "1+1=")
		// Filter CLAUDECODE only for the doctor inference probe to prevent
		// nested-session errors. Other subprocesses (scan/run/discuss/apply)
		// must preserve CLAUDECODE for the nested-session guard to work.
		if inferCmd.Env != nil {
			inferCmd.Env = platform.FilterEnv(inferCmd.Env, "CLAUDECODE")
		} else {
			inferCmd.Env = platform.FilterEnv(os.Environ(), "CLAUDECODE")
		}
		inferOut, inferErr := inferCmd.Output()
		inferCancel()
		inferOutput := string(inferOut)
		inferResult := checkClaudeInference(strings.TrimSpace(ExtractStreamResult(inferOutput)), inferErr)
		results = append(results, inferResult)

		// Context budget check: skip if inference failed
		if inferResult.Status != domain.CheckOK {
			results = append(results, domain.DoctorCheck{
				Name:    "context-budget",
				Status:  domain.CheckSkip,
				Message: "skipped (inference failed)",
			})
		} else {
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

	// --- skills-ref toolchain ---
	results = append(results, checkSkillsRefToolchain(baseDir, repair)...)

	// --- Repair: stale PID cleanup ---
	if repair {
		pidPath := filepath.Join(baseDir, domain.StateDir, "watch.pid")
		if data, err := os.ReadFile(pidPath); err == nil {
			pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
			if pid > 0 && !platform.IsProcessAlive(pid) {
				_ = os.Remove(pidPath)
				results = append(results, domain.DoctorCheck{
					Name: "stale-pid", Status: domain.CheckFixed,
					Message: "removed stale PID file",
				})
			}
		}
	}

	return results
}

// skillsRefBinNames lists possible binary names for the skills-ref package.
// "uv tool install skills-ref" installs as "agentskills", not "skills-ref".
var skillsRefBinNames = []string{"skills-ref", "agentskills"}

func findSkillsRefBin() (string, error) {
	for _, name := range skillsRefBinNames {
		if path, err := lookPath(name); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("none of %v found on PATH", skillsRefBinNames)
}

func checkSkillsRefToolchain(baseDir string, repair bool) []domain.DoctorCheck {
	if path, err := findSkillsRefBin(); err == nil {
		return []domain.DoctorCheck{{
			Name: "skills-ref", Status: domain.CheckOK,
			Message: fmt.Sprintf("skills-ref found on PATH (%s)", filepath.Base(path)),
		}}
	}
	_, uvErr := lookPath("uv")
	if uvErr != nil {
		return []domain.DoctorCheck{{
			Name: "skills-ref", Status: domain.CheckWarn,
			Message: "uv not found on PATH: SKILL.md spec validation is unavailable",
			Hint:    `install uv (https://docs.astral.sh/uv/) or "uv tool install skills-ref"`,
		}}
	}
	subDir := findSkillsRefDirFn(baseDir)
	if repair {
		if err := installSkillsRefFn(); err != nil {
			return []domain.DoctorCheck{{
				Name: "skills-ref", Status: domain.CheckWarn,
				Message: fmt.Sprintf("uv tool install skills-ref failed: %v", err),
				Hint:    `try manually: "uv tool install skills-ref"`,
			}}
		}
		// Verify that skills-ref is now actually on PATH after install.
		if _, err := findSkillsRefBin(); err != nil {
			return []domain.DoctorCheck{{
				Name: "skills-ref", Status: domain.CheckWarn,
				Message: "uv tool install succeeded but skills-ref still not on PATH",
				Hint:    `ensure uv tool bin directory is in PATH`,
			}}
		}
		return []domain.DoctorCheck{{
			Name: "skills-ref", Status: domain.CheckFixed,
			Message: "installed skills-ref via uv tool install",
		}}
	}
	hint := `run "sightjack doctor --repair" or "uv tool install skills-ref"`
	msg := "uv found but skills-ref not installed"
	if subDir != "" {
		msg = "uv found and checkout exists but skills-ref not on PATH"
		hint = `run "uv tool install skills-ref" or "sightjack doctor --repair"`
	}
	return []domain.DoctorCheck{{
		Name: "skills-ref", Status: domain.CheckWarn,
		Message: msg,
		Hint:    hint,
	}}
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
			case "skills":
				result.Hint = "review SKILL.md files in .siren/skills/ for unnecessary content"
			case "hooks":
				result.Hint = "review hook configurations for unnecessary output"
			default:
				result.Hint = ".claude/settings.json をプロジェクトに作成し、必要なプラグインのみ有効化を推奨"
			}
		}
	}
	return result
}
