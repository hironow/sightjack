//go:build scenario

package scenario_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestScenario_L1_EnvPrefixedClaudeCmd(t *testing.T) {
	if testing.Short() {
		t.Skip("scenario tests are not short")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	ws := NewWorkspace(t, "minimal")

	// Override config: set claude_cmd with env prefix
	fakeClaude := filepath.Join(ws.BinDir, "claude")
	envLogDir := t.TempDir()

	cfgPath := filepath.Join(ws.RepoPath, ".siren", "config.yaml")
	cfgData, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	var cfg map[string]any
	if err := yaml.Unmarshal(cfgData, &cfg); err != nil {
		t.Fatalf("parse config: %v", err)
	}

	// Set claude_cmd with env prefix — this is what we're testing
	cfg["claude_cmd"] = "CLAUDE_CONFIG_DIR=/tmp/test-config " + fakeClaude

	out, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(cfgPath, out, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// Add env log dir
	ws.Env = append(ws.Env, "FAKE_CLAUDE_ENV_LOG_DIR="+envLogDir)

	// Run doctor (exercises --version, mcp list, inference via fake-claude)
	err = ws.RunSightjack(t, ctx, "doctor", ws.RepoPath)
	if err != nil {
		t.Fatalf("sightjack doctor failed: %v", err)
	}

	// Verify env propagation
	entries, err := os.ReadDir(envLogDir)
	if err != nil {
		t.Fatalf("read env log dir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no env log files written — env vars not propagating to subprocess")
	}

	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(envLogDir, entry.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", entry.Name(), err)
		}

		var logged map[string]any
		if err := json.Unmarshal(data, &logged); err != nil {
			t.Fatalf("parse %s: %v", entry.Name(), err)
		}

		configDir, ok := logged["CLAUDE_CONFIG_DIR"]
		if !ok {
			t.Errorf("%s: CLAUDE_CONFIG_DIR not found in env log", entry.Name())
			continue
		}
		if configDir != "/tmp/test-config" {
			t.Errorf("%s: CLAUDE_CONFIG_DIR = %v, want /tmp/test-config", entry.Name(), configDir)
		}
	}

	t.Logf("verified CLAUDE_CONFIG_DIR propagation in %d invocations", len(entries))
}
