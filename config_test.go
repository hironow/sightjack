package sightjack

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_Defaults(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "sightjack.yaml")
	err := os.WriteFile(cfgPath, []byte(`
linear:
  team: "TEST-TEAM"
  project: "Test Project"
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(cfgPath)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Linear.Team != "TEST-TEAM" {
		t.Errorf("expected TEST-TEAM, got %s", cfg.Linear.Team)
	}
	if cfg.Linear.Project != "Test Project" {
		t.Errorf("expected Test Project, got %s", cfg.Linear.Project)
	}
	if cfg.Scan.ChunkSize != 20 {
		t.Errorf("expected default chunk_size 20, got %d", cfg.Scan.ChunkSize)
	}
	if cfg.Scan.MaxConcurrency != 3 {
		t.Errorf("expected default max_concurrency 3, got %d", cfg.Scan.MaxConcurrency)
	}
	if cfg.Claude.Command != "claude" {
		t.Errorf("expected default command 'claude', got %s", cfg.Claude.Command)
	}
	if cfg.Claude.TimeoutSec != 300 {
		t.Errorf("expected default timeout 300, got %d", cfg.Claude.TimeoutSec)
	}
	if cfg.Lang != "ja" {
		t.Errorf("expected default lang 'ja', got %s", cfg.Lang)
	}
}

func TestLoadConfig_FullOverride(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "sightjack.yaml")
	err := os.WriteFile(cfgPath, []byte(`
linear:
  team: "MY-TEAM"
  project: "My Project"
  cycle: "Sprint 5"
scan:
  chunk_size: 50
  max_concurrency: 5
claude:
  command: "cc-p"
  model: "sonnet"
  timeout_sec: 600
lang: "en"
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(cfgPath)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Scan.ChunkSize != 50 {
		t.Errorf("expected 50, got %d", cfg.Scan.ChunkSize)
	}
	if cfg.Claude.Model != "sonnet" {
		t.Errorf("expected sonnet, got %s", cfg.Claude.Model)
	}
	if cfg.Lang != "en" {
		t.Errorf("expected en, got %s", cfg.Lang)
	}
}

func TestLoadConfig_ZeroConcurrency_ClampsToOne(t *testing.T) {
	// given
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "sightjack.yaml")
	err := os.WriteFile(cfgPath, []byte(`
scan:
  max_concurrency: 0
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// when
	cfg, err := LoadConfig(cfgPath)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Scan.MaxConcurrency != 1 {
		t.Errorf("expected max_concurrency clamped to 1, got %d", cfg.Scan.MaxConcurrency)
	}
}

func TestLoadConfig_ZeroChunkSize_ClampsToDefault(t *testing.T) {
	// given
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "sightjack.yaml")
	err := os.WriteFile(cfgPath, []byte(`
scan:
  chunk_size: 0
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// when
	cfg, err := LoadConfig(cfgPath)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Scan.ChunkSize != 20 {
		t.Errorf("expected chunk_size clamped to default 20, got %d", cfg.Scan.ChunkSize)
	}
}

func TestLoadConfig_ZeroTimeout_ClampsToDefault(t *testing.T) {
	// given
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "sightjack.yaml")
	err := os.WriteFile(cfgPath, []byte(`
claude:
  timeout_sec: 0
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// when
	cfg, err := LoadConfig(cfgPath)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Claude.TimeoutSec != 300 {
		t.Errorf("expected timeout clamped to default 300, got %d", cfg.Claude.TimeoutSec)
	}
}

func TestLoadConfig_NegativeTimeout_ClampsToDefault(t *testing.T) {
	// given
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "sightjack.yaml")
	err := os.WriteFile(cfgPath, []byte(`
claude:
  timeout_sec: -10
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// when
	cfg, err := LoadConfig(cfgPath)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Claude.TimeoutSec != 300 {
		t.Errorf("expected timeout clamped to default 300, got %d", cfg.Claude.TimeoutSec)
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path.yaml")
	if err == nil {
		t.Error("expected error for missing config file")
	}
}

func TestDefaultConfig_ScribeEnabled(t *testing.T) {
	// given/when
	cfg := DefaultConfig()

	// then
	if !cfg.Scribe.Enabled {
		t.Error("expected Scribe.Enabled to be true by default")
	}
}

func TestLoadConfig_ScribeDisabled(t *testing.T) {
	// given
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "sightjack.yaml")
	err := os.WriteFile(cfgPath, []byte(`
scribe:
  enabled: false
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// when
	cfg, err := LoadConfig(cfgPath)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Scribe.Enabled {
		t.Error("expected Scribe.Enabled to be false")
	}
}

func TestDefaultConfig_StrictnessFog(t *testing.T) {
	// when
	cfg := DefaultConfig()

	// then
	if cfg.Strictness.Default != StrictnessFog {
		t.Errorf("expected fog, got %s", cfg.Strictness.Default)
	}
}

func TestLoadConfig_StrictnessAlert(t *testing.T) {
	// given
	dir := t.TempDir()
	path := filepath.Join(dir, "sightjack.yaml")
	os.WriteFile(path, []byte(`
linear:
  team: TEST
  project: Test
strictness:
  default: alert
`), 0644)

	// when
	cfg, err := LoadConfig(path)

	// then
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Strictness.Default != StrictnessAlert {
		t.Errorf("expected alert, got %s", cfg.Strictness.Default)
	}
}

func TestLoadConfig_StrictnessMissing_DefaultsFog(t *testing.T) {
	// given: config without strictness section
	dir := t.TempDir()
	path := filepath.Join(dir, "sightjack.yaml")
	os.WriteFile(path, []byte(`
linear:
  team: TEST
  project: Test
`), 0644)

	// when
	cfg, err := LoadConfig(path)

	// then
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Strictness.Default != StrictnessFog {
		t.Errorf("expected fog default, got %s", cfg.Strictness.Default)
	}
}

func TestLoadConfig_StrictnessInvalid_FallsBackToFog(t *testing.T) {
	// given: config with an invalid strictness value
	dir := t.TempDir()
	path := filepath.Join(dir, "sightjack.yaml")
	os.WriteFile(path, []byte(`
linear:
  team: TEST
  project: Test
strictness:
  default: banana
`), 0644)

	// when
	cfg, err := LoadConfig(path)

	// then
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Strictness.Default != StrictnessFog {
		t.Errorf("expected fog fallback for invalid value, got %s", cfg.Strictness.Default)
	}
}

func TestLoadConfig_StrictnessEmpty_FallsBackToFog(t *testing.T) {
	// given: config with empty strictness block
	dir := t.TempDir()
	path := filepath.Join(dir, "sightjack.yaml")
	os.WriteFile(path, []byte(`
linear:
  team: TEST
  project: Test
strictness:
`), 0644)

	// when
	cfg, err := LoadConfig(path)

	// then
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Strictness.Default != StrictnessFog {
		t.Errorf("expected fog fallback for empty strictness, got %s", cfg.Strictness.Default)
	}
}

func TestDoDTemplatesInDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.DoDTemplates != nil {
		t.Fatalf("expected nil DoDTemplates in default config, got %v", cfg.DoDTemplates)
	}
}

func TestLoadConfigWithDoDTemplates(t *testing.T) {
	content := `
linear:
  team: test
  project: test
dod_templates:
  auth:
    must:
      - "Unit tests for all public functions"
      - "Error handling for all API calls"
    should:
      - "Integration test coverage"
  infra:
    must:
      - "Terraform plan reviewed"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "sightjack.yaml")
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if len(cfg.DoDTemplates) != 2 {
		t.Fatalf("expected 2 DoD templates, got %d", len(cfg.DoDTemplates))
	}
	auth := cfg.DoDTemplates["auth"]
	if len(auth.Must) != 2 {
		t.Errorf("auth.Must: expected 2, got %d", len(auth.Must))
	}
	if len(auth.Should) != 1 {
		t.Errorf("auth.Should: expected 1, got %d", len(auth.Should))
	}
	infra := cfg.DoDTemplates["infra"]
	if len(infra.Must) != 1 {
		t.Errorf("infra.Must: expected 1, got %d", len(infra.Must))
	}
}

func TestLoadConfig_ScribeSectionMissing_DefaultsToEnabled(t *testing.T) {
	// given: config without scribe section
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "sightjack.yaml")
	err := os.WriteFile(cfgPath, []byte(`
linear:
  team: "TEST"
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// when
	cfg, err := LoadConfig(cfgPath)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.Scribe.Enabled {
		t.Error("expected Scribe.Enabled to default to true when section missing")
	}
}
