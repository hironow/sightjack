//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestE2E_Version(t *testing.T) {
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	dir := "/workspace/t_version"
	initTestRepo(t, ctx, c, dir)

	out, _, err := runCmd(t, ctx, c, dir, "version")
	if err != nil {
		t.Fatalf("version failed: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "sightjack v") {
		t.Errorf("expected version string, got: %s", out)
	}
}

func TestE2E_VersionJSON(t *testing.T) {
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	dir := "/workspace/t_version_json"
	initTestRepo(t, ctx, c, dir)

	out, _, err := runCmd(t, ctx, c, dir, "version", "--json")
	if err != nil {
		t.Fatalf("version --json failed: %v\noutput: %s", err, out)
	}
	var v map[string]string
	parseJSONOutput(t, out, &v)
	for _, key := range []string{"version", "commit", "date", "go"} {
		if _, ok := v[key]; !ok {
			t.Errorf("missing key %q in version JSON", key)
		}
	}
}

func TestE2E_Doctor(t *testing.T) {
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	dir := "/workspace/t_doctor"
	initTestRepo(t, ctx, c, dir)

	_, stderr, _ := runCmd(t, ctx, c, dir, "doctor")
	if len(strings.TrimSpace(stderr)) == 0 {
		t.Error("doctor output is empty")
	}
	if !strings.Contains(stderr, "doctor") {
		t.Errorf("doctor output missing header: %s", stderr)
	}
}

func TestE2E_Help(t *testing.T) {
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	dir := "/workspace/t_help"
	initTestRepo(t, ctx, c, dir)

	out, _, err := runCmd(t, ctx, c, dir, "--help")
	if err != nil {
		t.Fatalf("--help failed: %v\noutput: %s", err, out)
	}
	for _, sub := range []string{"mcp", "sessions", "show", "status", "doctor", "version"} {
		if !strings.Contains(out, sub) {
			t.Errorf("--help output missing subcommand %q", sub)
		}
	}
}

func TestE2E_UnknownCommand(t *testing.T) {
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	dir := "/workspace/t_unknown"
	initTestRepo(t, ctx, c, dir)

	_, _, err := runCmd(t, ctx, c, dir, "nonexistent")
	if err == nil {
		t.Error("expected error for unknown command")
	}
}

func TestE2E_Show_NoState(t *testing.T) {
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	dir := "/workspace/t_show_nostate"
	initTestRepo(t, ctx, c, dir)

	// We don't have .siren/events/ in t_show_nostate, but initTestRepo created basic repo.
	// Let's run show on a different nonexistent state dir.
	_, _, err := runCmd(t, ctx, c, dir, "show", "/workspace/nonexistent-state-dir")
	if err == nil {
		t.Error("expected error when no state exists")
	}
}

func TestE2E_ArchivePrune_Empty(t *testing.T) {
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	dir := "/workspace/t_archive_prune"
	initTestRepo(t, ctx, c, dir)

	out, _, err := runCmd(t, ctx, c, dir, "archive-prune", dir)
	if err != nil {
		t.Fatalf("archive-prune failed: %v\noutput: %s", err, out)
	}
}

func TestE2E_Init_WithFlags(t *testing.T) {
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	dir := "/workspace/t_init_flags"
	
	// Create folder and git init
	execInContainer(t, ctx, c, []string{"mkdir", "-p", dir})
	execInContainer(t, ctx, c, []string{"sh", "-c", fmt.Sprintf("cd %s && git init --initial-branch=main", dir)})

	out, _, err := runCmd(t, ctx, c, dir, "init", "--team", "TestTeam", "--project", "TestProject", dir)
	if err != nil {
		t.Fatalf("init failed: %v\noutput: %s", err, out)
	}
	cfgFile := fmt.Sprintf("%s/.siren/config.yaml", dir)
	if !fileExistsInContainer(t, ctx, c, cfgFile) {
		t.Errorf("config file not created: %s", cfgFile)
	}
}

func TestE2E_MCPServerToolsList(t *testing.T) {
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	dir := "/workspace/t_mcp"
	initTestRepo(t, ctx, c, dir)

	input := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	stdout, _, err := runCmdStdin(t, ctx, c, dir, input, "mcp")
	if err != nil {
		t.Fatalf("mcp command failed: %v", err)
	}

	idx := strings.Index(stdout, `{"jsonrpc"`)
	if idx < 0 {
		t.Fatalf("no JSON-RPC response found in stdout: %s", stdout)
	}
	jsonStr := stdout[idx:]

	var resp struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Result  struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		t.Fatalf("failed to unmarshal JSON-RPC response: %v\nraw: %s", err, jsonStr)
	}

	if resp.JSONRPC != "2.0" {
		t.Errorf("expected jsonrpc 2.0, got %s", resp.JSONRPC)
	}

	if resp.ID != 1 {
		t.Errorf("expected id 1, got %d", resp.ID)
	}

	expectedTools := map[string]bool{
		"sightjack.ping":             false,
		"sightjack.next_wave":        false,
		"sightjack.get_scan_result":  false,
		"sightjack.update_strictness": false,
	}

	for _, tool := range resp.Result.Tools {
		if _, ok := expectedTools[tool.Name]; ok {
			expectedTools[tool.Name] = true
		}
	}

	for name, found := range expectedTools {
		if !found {
			t.Errorf("missing expected tool in MCP response: %s", name)
		}
	}
}
