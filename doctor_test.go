package sightjack_test

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/sightjack"
	"github.com/hironow/sightjack/internal/session"
)

func TestCheckConfig_ValidConfig(t *testing.T) {
	// given: valid config file
	dir := t.TempDir()
	path := filepath.Join(dir, "sightjack.yaml")
	os.WriteFile(path, []byte(`
linear:
  team: "Test"
  project: "Project"
`), 0644)

	// when
	result := session.CheckConfig(path)

	// then
	if result.Status != session.CheckOK {
		t.Errorf("expected CheckOK, got %v: %s", result.Status, result.Message)
	}
	if result.Name != "Config" {
		t.Errorf("expected name 'Config', got %q", result.Name)
	}
}

func TestCheckConfig_MissingFile(t *testing.T) {
	// given: nonexistent path
	result := session.CheckConfig("/nonexistent/sightjack.yaml")

	// then
	if result.Status != session.CheckFail {
		t.Errorf("expected CheckFail, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckConfig_InvalidYAML(t *testing.T) {
	// given: invalid YAML content
	dir := t.TempDir()
	path := filepath.Join(dir, "sightjack.yaml")
	os.WriteFile(path, []byte(`{{{invalid yaml`), 0644)

	// when
	result := session.CheckConfig(path)

	// then
	if result.Status != session.CheckFail {
		t.Errorf("expected CheckFail for invalid YAML, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckTool_Exists(t *testing.T) {
	// given: "git" is guaranteed to exist in dev environment and supports --version
	ctx := context.Background()

	// when
	result := session.CheckTool(ctx, "git")

	// then
	if result.Status != session.CheckOK {
		t.Errorf("expected CheckOK for 'git', got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "git") {
		t.Errorf("expected message to contain path, got: %s", result.Message)
	}
}

func TestCheckTool_NotFound(t *testing.T) {
	// given: nonexistent tool
	ctx := context.Background()

	// when
	result := session.CheckTool(ctx, "nonexistent-tool-xyz-12345")

	// then
	if result.Status != session.CheckFail {
		t.Errorf("expected CheckFail, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckStatusLabel(t *testing.T) {
	tests := []struct {
		status session.CheckStatus
		want   string
	}{
		{session.CheckOK, "OK"},
		{session.CheckFail, "FAIL"},
		{session.CheckSkip, "SKIP"},
	}
	for _, tt := range tests {
		if got := tt.status.StatusLabel(); got != tt.want {
			t.Errorf("StatusLabel(%d): expected %q, got %q", tt.status, tt.want, got)
		}
	}
}

// --- CheckClaudeAuth tests ---

func TestCheckClaudeAuth_Success(t *testing.T) {
	// given: mock claude that responds OK
	cleanup := session.OverrideNewCmd(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "echo", "OK")
	})
	defer cleanup()

	cfg := &sightjack.Config{
		Claude: sightjack.ClaudeConfig{Command: "claude", TimeoutSec: 10},
		Retry:  sightjack.RetryConfig{MaxAttempts: 1, BaseDelaySec: 0},
	}
	ctx := context.Background()

	// when
	result := session.CheckClaudeAuth(ctx, cfg, sightjack.NewLogger(io.Discard, false))

	// then
	if result.Status != session.CheckOK {
		t.Errorf("expected CheckOK, got %v: %s", result.Status, result.Message)
	}
	if result.Name != "Claude Auth" {
		t.Errorf("expected name 'Claude Auth', got %q", result.Name)
	}
}

func TestCheckClaudeAuth_NotLoggedIn(t *testing.T) {
	// given: mock claude that outputs "Not logged in" and exits 1
	cleanup := session.OverrideNewCmd(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", "-c", `echo "Not logged in · Please run /login"; exit 1`)
	})
	defer cleanup()

	cfg := &sightjack.Config{
		Claude: sightjack.ClaudeConfig{Command: "claude", TimeoutSec: 10},
		Retry:  sightjack.RetryConfig{MaxAttempts: 1, BaseDelaySec: 0},
	}
	ctx := context.Background()

	// when
	result := session.CheckClaudeAuth(ctx, cfg, sightjack.NewLogger(io.Discard, false))

	// then
	if result.Status != session.CheckFail {
		t.Errorf("expected CheckFail, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Hint, "claude login") {
		t.Errorf("expected Hint to contain 'claude login', got: %s", result.Hint)
	}
}

func TestCheckClaudeAuth_OtherFailure(t *testing.T) {
	// given: mock claude that fails with unknown error
	cleanup := session.OverrideNewCmd(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "false")
	})
	defer cleanup()

	cfg := &sightjack.Config{
		Claude: sightjack.ClaudeConfig{Command: "claude", TimeoutSec: 10},
		Retry:  sightjack.RetryConfig{MaxAttempts: 1, BaseDelaySec: 0},
	}
	ctx := context.Background()

	// when
	result := session.CheckClaudeAuth(ctx, cfg, sightjack.NewLogger(io.Discard, false))

	// then
	if result.Status != session.CheckFail {
		t.Errorf("expected CheckFail, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckClaudeAuth_NilConfig_Skips(t *testing.T) {
	// given: nil config
	ctx := context.Background()

	// when
	result := session.CheckClaudeAuth(ctx, nil, sightjack.NewLogger(io.Discard, false))

	// then
	if result.Status != session.CheckSkip {
		t.Errorf("expected CheckSkip, got %v: %s", result.Status, result.Message)
	}
}

// --- CheckLinearMCP tests ---

func TestCheckLinearMCP_Success(t *testing.T) {
	// given: mock claude that returns team info
	cleanup := session.OverrideNewCmd(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "echo", `{"teams": [{"name": "Engineering"}]}`)
	})
	defer cleanup()

	cfg := &sightjack.Config{
		Claude: sightjack.ClaudeConfig{Command: "claude", TimeoutSec: 10},
		Linear: sightjack.LinearConfig{Team: "Engineering"},
		Retry:  sightjack.RetryConfig{MaxAttempts: 1, BaseDelaySec: 0},
	}
	ctx := context.Background()

	// when
	result := session.CheckLinearMCP(ctx, cfg, sightjack.NewLogger(io.Discard, false))

	// then
	if result.Status != session.CheckOK {
		t.Errorf("expected CheckOK, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckLinearMCP_Failure(t *testing.T) {
	// given: mock claude that fails (auth is OK but MCP fails)
	cleanup := session.OverrideNewCmd(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "false")
	})
	defer cleanup()

	cfg := &sightjack.Config{
		Claude: sightjack.ClaudeConfig{Command: "claude", TimeoutSec: 10},
		Linear: sightjack.LinearConfig{Team: "Engineering"},
		Retry:  sightjack.RetryConfig{MaxAttempts: 1, BaseDelaySec: 0},
	}
	ctx := context.Background()

	// when
	result := session.CheckLinearMCP(ctx, cfg, sightjack.NewLogger(io.Discard, false))

	// then
	if result.Status != session.CheckFail {
		t.Errorf("expected CheckFail, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Hint, "claude mcp add") {
		t.Errorf("expected Hint to contain 'claude mcp add', got: %s", result.Hint)
	}
}

func TestCheckLinearMCP_NilConfig_Skips(t *testing.T) {
	// given: nil config (config load failed)
	ctx := context.Background()

	// when
	result := session.CheckLinearMCP(ctx, nil, sightjack.NewLogger(io.Discard, false))

	// then
	if result.Status != session.CheckSkip {
		t.Errorf("expected CheckSkip, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckStateDir_Writable(t *testing.T) {
	// given: a directory where .siren/ can be created
	dir := t.TempDir()

	// when
	result := session.CheckStateDir(dir)

	// then
	if result.Status != session.CheckOK {
		t.Errorf("expected CheckOK, got %v: %s", result.Status, result.Message)
	}
	if result.Name != "State Dir" {
		t.Errorf("expected name 'State Dir', got %q", result.Name)
	}
}

func TestCheckStateDir_NotWritable(t *testing.T) {
	// given: a read-only directory
	dir := t.TempDir()
	os.Chmod(dir, 0555)
	defer os.Chmod(dir, 0755) // cleanup

	// when
	result := session.CheckStateDir(dir)

	// then
	if result.Status != session.CheckFail {
		t.Errorf("expected CheckFail for read-only dir, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckStateDir_ExistingDir(t *testing.T) {
	// given: .siren/ already exists and is writable
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".siren"), 0755)

	// when
	result := session.CheckStateDir(dir)

	// then
	if result.Status != session.CheckOK {
		t.Errorf("expected CheckOK for existing .siren/, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckSkills_OK(t *testing.T) {
	// given: valid SKILL.md files installed
	baseDir := t.TempDir()
	if err := session.InstallSkills(baseDir, sightjack.SkillsFS); err != nil {
		t.Fatalf("InstallSkills: %v", err)
	}

	// when
	result := session.CheckSkills(baseDir)

	// then
	if result.Status != session.CheckOK {
		t.Errorf("expected CheckOK, got %v: %s", result.Status, result.Message)
	}
	if result.Name != "Skills" {
		t.Errorf("expected name 'Skills', got %q", result.Name)
	}
}

func TestCheckSkills_Missing(t *testing.T) {
	// given: empty dir (no skills installed)
	baseDir := t.TempDir()

	// when
	result := session.CheckSkills(baseDir)

	// then
	if result.Status != session.CheckFail {
		t.Errorf("expected CheckFail for missing SKILL.md files, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckSkills_MissingSchemaVersion(t *testing.T) {
	// given: SKILL.md files exist but lack dmail-schema-version
	baseDir := t.TempDir()
	skillsDir := filepath.Join(baseDir, ".siren", "skills")
	os.MkdirAll(filepath.Join(skillsDir, "dmail-sendable"), 0755)
	os.MkdirAll(filepath.Join(skillsDir, "dmail-readable"), 0755)
	os.WriteFile(filepath.Join(skillsDir, "dmail-sendable", "SKILL.md"), []byte("---\nname: dmail-sendable\n---\n"), 0644)
	os.WriteFile(filepath.Join(skillsDir, "dmail-readable", "SKILL.md"), []byte("---\nname: dmail-readable\n---\n"), 0644)

	// when
	result := session.CheckSkills(baseDir)

	// then
	if result.Status != session.CheckFail {
		t.Errorf("expected CheckFail for missing schema version, got %v: %s", result.Status, result.Message)
	}
}

func TestRunDoctor_ConfigFailure_ClaudeAuthAndMCPSkipped(t *testing.T) {
	// given: nonexistent config path → config check fails, cfg=nil
	cleanup := session.OverrideNewCmd(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "echo", "ok")
	})
	defer cleanup()

	dir := t.TempDir()
	ctx := context.Background()

	// when
	results := session.RunDoctor(ctx, "/nonexistent/sightjack.yaml", dir, sightjack.NewLogger(io.Discard, false))

	// then: should have 8 results
	if len(results) != 8 {
		t.Fatalf("expected 8 results, got %d", len(results))
	}
	// Config should fail
	if results[0].Status != session.CheckFail {
		t.Errorf("Config: expected FAIL, got %v", results[0].Status)
	}
	// Claude Auth should be skipped (nil config)
	auth := results[5]
	if auth.Name != "Claude Auth" {
		t.Errorf("expected 'Claude Auth', got %q", auth.Name)
	}
	if auth.Status != session.CheckSkip {
		t.Errorf("Claude Auth: expected SKIP (nil config), got %v: %s", auth.Status, auth.Message)
	}
	// Linear MCP should be skipped (nil config)
	mcp := results[6]
	if mcp.Name != "Linear MCP" {
		t.Errorf("expected 'Linear MCP', got %q", mcp.Name)
	}
	if mcp.Status != session.CheckSkip {
		t.Errorf("Linear MCP: expected SKIP (nil config), got %v: %s", mcp.Status, mcp.Message)
	}
}

func TestRunDoctor_ClaudeUnavailable_AuthAndMCPSkipped(t *testing.T) {
	// given: claude binary does not exist → Claude Auth + Linear MCP should be skipped
	cleanup := session.OverrideNewCmd(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "echo", "ok")
	})
	defer cleanup()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "sightjack.yaml")
	os.WriteFile(cfgPath, []byte(`
linear:
  team: "Test"
  project: "Project"
claude:
  command: "nonexistent-claude-binary-xyz"
`), 0644)

	ctx := context.Background()

	// when
	results := session.RunDoctor(ctx, cfgPath, dir, sightjack.NewLogger(io.Discard, false))

	// then
	if len(results) != 8 {
		t.Fatalf("expected 8 results, got %d", len(results))
	}
	// Config should pass
	if results[0].Status != session.CheckOK {
		t.Errorf("Config: expected OK, got %v", results[0].Status)
	}
	// claude binary check should fail (nonexistent binary)
	if results[2].Status != session.CheckFail {
		t.Errorf("claude: expected FAIL, got %v: %s", results[2].Status, results[2].Message)
	}
	// Claude Auth should be skipped because claude binary is unavailable
	auth := results[5]
	if auth.Status != session.CheckSkip {
		t.Errorf("Claude Auth: expected SKIP, got %v: %s", auth.Status, auth.Message)
	}
	if !strings.Contains(auth.Message, "claude not available") {
		t.Errorf("expected 'claude not available' in message, got: %s", auth.Message)
	}
	// Linear MCP should be skipped because claude binary is unavailable
	mcp := results[6]
	if mcp.Status != session.CheckSkip {
		t.Errorf("Linear MCP: expected SKIP, got %v: %s", mcp.Status, mcp.Message)
	}
}

func TestRunDoctor_ReturnsAllResults(t *testing.T) {
	// given: mock claude for auth + MCP checks
	cleanup := session.OverrideNewCmd(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "echo", "ok")
	})
	defer cleanup()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "sightjack.yaml")
	os.WriteFile(cfgPath, []byte(`
linear:
  team: "Test"
  project: "Project"
`), 0644)

	ctx := context.Background()

	// when
	results := session.RunDoctor(ctx, cfgPath, dir, sightjack.NewLogger(io.Discard, false))

	// then: should have 8 results (config, state dir, claude, git, skills, claude auth, linear mcp, success-rate)
	if len(results) != 8 {
		t.Fatalf("expected 8 results, got %d: %v", len(results), results)
	}
	if results[0].Name != "Config" || results[0].Status != session.CheckOK {
		t.Errorf("Config check: expected OK, got %v: %s", results[0].Status, results[0].Message)
	}
	if results[1].Name != "State Dir" || results[1].Status != session.CheckOK {
		t.Errorf("State Dir check: expected OK, got %v: %s", results[1].Status, results[1].Message)
	}
	if results[5].Name != "Claude Auth" || results[5].Status != session.CheckOK {
		t.Errorf("Claude Auth check: expected OK, got %v: %s", results[5].Status, results[5].Message)
	}
	if results[6].Name != "Linear MCP" || results[6].Status != session.CheckOK {
		t.Errorf("Linear MCP check: expected OK, got %v: %s", results[6].Status, results[6].Message)
	}
	// success-rate should be last result, OK with "no events" (no events dir in temp)
	sr := results[7]
	if sr.Name != "success-rate" {
		t.Errorf("expected 'success-rate', got %q", sr.Name)
	}
	if sr.Status != session.CheckOK {
		t.Errorf("success-rate: expected OK, got %v: %s", sr.Status, sr.Message)
	}
	if sr.Message != "no events" {
		t.Errorf("success-rate: expected 'no events', got %q", sr.Message)
	}
}

func TestRunDoctor_SuccessRateWithEvents(t *testing.T) {
	// given: mock claude, valid config, and event data in .siren/events/{session-id}/
	cleanup := session.OverrideNewCmd(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "echo", "ok")
	})
	defer cleanup()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "sightjack.yaml")
	os.WriteFile(cfgPath, []byte(`
linear:
  team: "Test"
  project: "Project"
`), 0644)

	// Create event store with 2 applied and 1 rejected wave events
	sessionDir := filepath.Join(dir, ".siren", "events", "test-session-001")
	os.MkdirAll(sessionDir, 0755)
	events := strings.Join([]string{
		`{"id":"e1","type":"wave_applied","timestamp":"2026-03-01T10:00:00Z","data":{"wave_id":"w1","cluster_name":"c1","applied":1,"total_count":1},"session_id":"test-session-001"}`,
		`{"id":"e2","type":"wave_applied","timestamp":"2026-03-01T10:01:00Z","data":{"wave_id":"w2","cluster_name":"c1","applied":1,"total_count":1},"session_id":"test-session-001"}`,
		`{"id":"e3","type":"wave_rejected","timestamp":"2026-03-01T10:02:00Z","data":{"wave_id":"w3","cluster_name":"c1"},"session_id":"test-session-001"}`,
	}, "\n") + "\n"
	os.WriteFile(filepath.Join(sessionDir, "2026-03-01.jsonl"), []byte(events), 0644)

	ctx := context.Background()

	// when
	results := session.RunDoctor(ctx, cfgPath, dir, sightjack.NewLogger(io.Discard, false))

	// then: find success-rate result
	var found bool
	for _, r := range results {
		if r.Name == "success-rate" {
			found = true
			if r.Status != session.CheckOK {
				t.Errorf("success-rate: expected OK, got %v", r.Status)
			}
			want := "66.7% (2/3)"
			if r.Message != want {
				t.Errorf("success-rate: got %q, want %q", r.Message, want)
			}
			break
		}
	}
	if !found {
		t.Errorf("success-rate check not found in results (got %d results)", len(results))
	}
}
