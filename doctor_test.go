package sightjack

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
	result := checkConfig(path)

	// then
	if result.Status != CheckOK {
		t.Errorf("expected CheckOK, got %v: %s", result.Status, result.Message)
	}
	if result.Name != "Config" {
		t.Errorf("expected name 'Config', got %q", result.Name)
	}
}

func TestCheckConfig_MissingFile(t *testing.T) {
	// given: nonexistent path
	result := checkConfig("/nonexistent/sightjack.yaml")

	// then
	if result.Status != CheckFail {
		t.Errorf("expected CheckFail, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckConfig_InvalidYAML(t *testing.T) {
	// given: invalid YAML content
	dir := t.TempDir()
	path := filepath.Join(dir, "sightjack.yaml")
	os.WriteFile(path, []byte(`{{{invalid yaml`), 0644)

	// when
	result := checkConfig(path)

	// then
	if result.Status != CheckFail {
		t.Errorf("expected CheckFail for invalid YAML, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckTool_Exists(t *testing.T) {
	// given: "git" is guaranteed to exist in dev environment and supports --version
	ctx := context.Background()

	// when
	result := checkTool(ctx, "git")

	// then
	if result.Status != CheckOK {
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
	result := checkTool(ctx, "nonexistent-tool-xyz-12345")

	// then
	if result.Status != CheckFail {
		t.Errorf("expected CheckFail, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckStatusLabel(t *testing.T) {
	tests := []struct {
		status CheckStatus
		want   string
	}{
		{CheckOK, "OK"},
		{CheckFail, "FAIL"},
		{CheckSkip, "SKIP"},
	}
	for _, tt := range tests {
		if got := tt.status.StatusLabel(); got != tt.want {
			t.Errorf("StatusLabel(%d): expected %q, got %q", tt.status, tt.want, got)
		}
	}
}

func TestCheckLinearMCP_Success(t *testing.T) {
	// given: mock claude that returns team info
	newCmd = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("echo", `{"teams": [{"name": "Engineering"}]}`)
	}
	defer func() { newCmd = defaultNewCmd }()

	cfg := &Config{
		Claude: ClaudeConfig{Command: "claude", TimeoutSec: 10},
		Linear: LinearConfig{Team: "Engineering"},
		Retry:  RetryConfig{MaxAttempts: 1, BaseDelaySec: 0},
	}
	ctx := context.Background()

	// when
	result := checkLinearMCP(ctx, cfg)

	// then
	if result.Status != CheckOK {
		t.Errorf("expected CheckOK, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckLinearMCP_Failure(t *testing.T) {
	// given: mock claude that fails
	newCmd = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("false")
	}
	defer func() { newCmd = defaultNewCmd }()

	cfg := &Config{
		Claude: ClaudeConfig{Command: "claude", TimeoutSec: 10},
		Linear: LinearConfig{Team: "Engineering"},
		Retry:  RetryConfig{MaxAttempts: 1, BaseDelaySec: 0},
	}
	ctx := context.Background()

	// when
	result := checkLinearMCP(ctx, cfg)

	// then
	if result.Status != CheckFail {
		t.Errorf("expected CheckFail, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckLinearMCP_NilConfig_Skips(t *testing.T) {
	// given: nil config (config load failed)
	ctx := context.Background()

	// when
	result := checkLinearMCP(ctx, nil)

	// then
	if result.Status != CheckSkip {
		t.Errorf("expected CheckSkip, got %v: %s", result.Status, result.Message)
	}
}

func TestRunDoctor_ConfigFailure_LinearMCPSkipped(t *testing.T) {
	// given: nonexistent config path → config check fails, cfg=nil
	newCmd = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "ok")
	}
	defer func() { newCmd = defaultNewCmd }()

	ctx := context.Background()

	// when
	results := RunDoctor(ctx, "/nonexistent/sightjack.yaml")

	// then: should have 4 results
	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}
	// Config should fail
	if results[0].Status != CheckFail {
		t.Errorf("Config: expected FAIL, got %v", results[0].Status)
	}
	// Linear MCP should be OK or Skip depending on cfg=nil path
	// When cfg is nil, checkLinearMCP returns Skip
	mcp := results[3]
	if mcp.Name != "Linear MCP" {
		t.Errorf("expected 'Linear MCP', got %q", mcp.Name)
	}
	if mcp.Status != CheckSkip {
		t.Errorf("Linear MCP: expected SKIP (nil config), got %v: %s", mcp.Status, mcp.Message)
	}
}

func TestRunDoctor_ClaudeUnavailable_LinearMCPSkipped(t *testing.T) {
	// given: claude binary does not exist → Linear MCP should be skipped
	newCmd = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "ok")
	}
	defer func() { newCmd = defaultNewCmd }()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "sightjack.yaml")
	// Set claude command to a nonexistent binary
	os.WriteFile(cfgPath, []byte(`
linear:
  team: "Test"
  project: "Project"
claude:
  command: "nonexistent-claude-binary-xyz"
`), 0644)

	ctx := context.Background()

	// when
	results := RunDoctor(ctx, cfgPath)

	// then
	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}
	// Config should pass
	if results[0].Status != CheckOK {
		t.Errorf("Config: expected OK, got %v", results[0].Status)
	}
	// claude check should fail (nonexistent binary)
	if results[1].Status != CheckFail {
		t.Errorf("claude: expected FAIL, got %v: %s", results[1].Status, results[1].Message)
	}
	// Linear MCP should be skipped because claude is unavailable
	mcp := results[3]
	if mcp.Status != CheckSkip {
		t.Errorf("Linear MCP: expected SKIP, got %v: %s", mcp.Status, mcp.Message)
	}
	if !strings.Contains(mcp.Message, "claude not available") {
		t.Errorf("expected 'claude not available' in message, got: %s", mcp.Message)
	}
}

func TestRunDoctor_ReturnsAllResults(t *testing.T) {
	// given: mock claude for MCP check
	newCmd = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "ok")
	}
	defer func() { newCmd = defaultNewCmd }()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "sightjack.yaml")
	os.WriteFile(cfgPath, []byte(`
linear:
  team: "Test"
  project: "Project"
`), 0644)

	ctx := context.Background()

	// when
	results := RunDoctor(ctx, cfgPath)

	// then: should have 4 results (config, claude, git, linear mcp)
	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d: %v", len(results), results)
	}
	// Config check should succeed
	if results[0].Name != "Config" || results[0].Status != CheckOK {
		t.Errorf("Config check: expected OK, got %v: %s", results[0].Status, results[0].Message)
	}
}
