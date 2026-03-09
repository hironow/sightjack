package session_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/session"
)

func TestLoadConfig_Defaults(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "sightjack.yaml")
	err := os.WriteFile(cfgPath, []byte(`
tracker:
  team: "TEST-TEAM"
  project: "Test Project"
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := session.LoadConfig(cfgPath)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Tracker.Team != "TEST-TEAM" {
		t.Errorf("expected TEST-TEAM, got %s", cfg.Tracker.Team)
	}
	if cfg.Tracker.Project != "Test Project" {
		t.Errorf("expected Test Project, got %s", cfg.Tracker.Project)
	}
	if cfg.Scan.ChunkSize != 20 {
		t.Errorf("expected default chunk_size 20, got %d", cfg.Scan.ChunkSize)
	}
	if cfg.Scan.MaxConcurrency != 3 {
		t.Errorf("expected default max_concurrency 3, got %d", cfg.Scan.MaxConcurrency)
	}
	if cfg.Assistant.Command != "claude" {
		t.Errorf("expected default command 'claude', got %s", cfg.Assistant.Command)
	}
	if cfg.Assistant.TimeoutSec != 300 {
		t.Errorf("expected default timeout 300, got %d", cfg.Assistant.TimeoutSec)
	}
	if cfg.Lang != "ja" {
		t.Errorf("expected default lang 'ja', got %s", cfg.Lang)
	}
}

func TestLoadConfig_FullOverride(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "sightjack.yaml")
	err := os.WriteFile(cfgPath, []byte(`
tracker:
  team: "MY-TEAM"
  project: "My Project"
  cycle: "Sprint 5"
scan:
  chunk_size: 50
  max_concurrency: 5
assistant:
  command: "cc-p"
  model: "sonnet"
  timeout_sec: 600
lang: "en"
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := session.LoadConfig(cfgPath)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Scan.ChunkSize != 50 {
		t.Errorf("expected 50, got %d", cfg.Scan.ChunkSize)
	}
	if cfg.Assistant.Model != "sonnet" {
		t.Errorf("expected sonnet, got %s", cfg.Assistant.Model)
	}
	if cfg.Lang != "en" {
		t.Errorf("expected en, got %s", cfg.Lang)
	}
}

func TestLoadConfig_ZeroConcurrency_ClampsToOne(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "sightjack.yaml")
	err := os.WriteFile(cfgPath, []byte(`
scan:
  max_concurrency: 0
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := session.LoadConfig(cfgPath)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Scan.MaxConcurrency != 1 {
		t.Errorf("expected max_concurrency clamped to 1, got %d", cfg.Scan.MaxConcurrency)
	}
}

func TestLoadConfig_ZeroChunkSize_ClampsToDefault(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "sightjack.yaml")
	err := os.WriteFile(cfgPath, []byte(`
scan:
  chunk_size: 0
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := session.LoadConfig(cfgPath)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Scan.ChunkSize != 20 {
		t.Errorf("expected chunk_size clamped to default 20, got %d", cfg.Scan.ChunkSize)
	}
}

func TestLoadConfig_ZeroTimeout_ClampsToDefault(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "sightjack.yaml")
	err := os.WriteFile(cfgPath, []byte(`
assistant:
  timeout_sec: 0
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := session.LoadConfig(cfgPath)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Assistant.TimeoutSec != 300 {
		t.Errorf("expected timeout clamped to default 300, got %d", cfg.Assistant.TimeoutSec)
	}
}

func TestLoadConfig_NegativeTimeout_ClampsToDefault(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "sightjack.yaml")
	err := os.WriteFile(cfgPath, []byte(`
assistant:
  timeout_sec: -10
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := session.LoadConfig(cfgPath)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Assistant.TimeoutSec != 300 {
		t.Errorf("expected timeout clamped to default 300, got %d", cfg.Assistant.TimeoutSec)
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := session.LoadConfig("/nonexistent/path.yaml")
	if err == nil {
		t.Error("expected error for missing config file")
	}
}

func TestLoadConfig_ScribeDisabled(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "sightjack.yaml")
	err := os.WriteFile(cfgPath, []byte(`
scribe:
  enabled: false
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := session.LoadConfig(cfgPath)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Scribe.Enabled {
		t.Error("expected Scribe.Enabled to be false")
	}
}

func TestLoadConfig_StrictnessAlert(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sightjack.yaml")
	os.WriteFile(path, []byte(`
tracker:
  team: TEST
  project: Test
strictness:
  default: alert
`), 0644)

	cfg, err := session.LoadConfig(path)

	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Strictness.Default != domain.StrictnessAlert {
		t.Errorf("expected alert, got %s", cfg.Strictness.Default)
	}
}

func TestLoadConfig_StrictnessMissing_DefaultsFog(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sightjack.yaml")
	os.WriteFile(path, []byte(`
tracker:
  team: TEST
  project: Test
`), 0644)

	cfg, err := session.LoadConfig(path)

	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Strictness.Default != domain.StrictnessFog {
		t.Errorf("expected fog default, got %s", cfg.Strictness.Default)
	}
}

func TestLoadConfig_StrictnessInvalid_FallsBackToFog(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sightjack.yaml")
	os.WriteFile(path, []byte(`
tracker:
  team: TEST
  project: Test
strictness:
  default: banana
`), 0644)

	cfg, err := session.LoadConfig(path)

	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Strictness.Default != domain.StrictnessFog {
		t.Errorf("expected fog fallback for invalid value, got %s", cfg.Strictness.Default)
	}
}

func TestLoadConfig_StrictnessEmpty_FallsBackToFog(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sightjack.yaml")
	os.WriteFile(path, []byte(`
tracker:
  team: TEST
  project: Test
strictness:
`), 0644)

	cfg, err := session.LoadConfig(path)

	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Strictness.Default != domain.StrictnessFog {
		t.Errorf("expected fog fallback for empty strictness, got %s", cfg.Strictness.Default)
	}
}

func TestLoadConfigWithDoDTemplates(t *testing.T) {
	content := `
tracker:
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

	cfg, err := session.LoadConfig(path)
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

func TestLoadConfigWithRetry(t *testing.T) {
	content := `
tracker:
  team: test
  project: test
retry:
  max_attempts: 5
  base_delay_sec: 1
`
	dir := t.TempDir()
	path := filepath.Join(dir, "sightjack.yaml")
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := session.LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Retry.MaxAttempts != 5 {
		t.Errorf("MaxAttempts: expected 5, got %d", cfg.Retry.MaxAttempts)
	}
	if cfg.Retry.BaseDelaySec != 1 {
		t.Errorf("BaseDelaySec: expected 1, got %d", cfg.Retry.BaseDelaySec)
	}
}

func TestLoadConfigRetryValidation(t *testing.T) {
	content := `
tracker:
  team: test
  project: test
retry:
  max_attempts: 0
  base_delay_sec: -1
`
	dir := t.TempDir()
	path := filepath.Join(dir, "sightjack.yaml")
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := session.LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Retry.MaxAttempts != 3 {
		t.Errorf("expected corrected MaxAttempts=3, got %d", cfg.Retry.MaxAttempts)
	}
	if cfg.Retry.BaseDelaySec != 2 {
		t.Errorf("expected corrected BaseDelaySec=2, got %d", cfg.Retry.BaseDelaySec)
	}
}

func TestLoadConfigWithLabels(t *testing.T) {
	content := `
tracker:
  team: test
  project: test
labels:
  enabled: false
  prefix: "myprefix"
  ready_label: "myprefix:done"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "sightjack.yaml")
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := session.LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Labels.Enabled {
		t.Error("expected Labels.Enabled=false")
	}
	if cfg.Labels.Prefix != "myprefix" {
		t.Errorf("Prefix: expected 'myprefix', got %q", cfg.Labels.Prefix)
	}
	if cfg.Labels.ReadyLabel != "myprefix:done" {
		t.Errorf("ReadyLabel: expected 'myprefix:done', got %q", cfg.Labels.ReadyLabel)
	}
}

func TestLoadConfigLabelsEnabled_EmptyValues_FallsBackToDefaults(t *testing.T) {
	content := `
tracker:
  team: test
  project: test
labels:
  enabled: true
  prefix: ""
  ready_label: ""
`
	dir := t.TempDir()
	path := filepath.Join(dir, "sightjack.yaml")
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := session.LoadConfig(path)

	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Labels.Prefix != "sightjack" {
		t.Errorf("expected default prefix 'sightjack', got %q", cfg.Labels.Prefix)
	}
	if cfg.Labels.ReadyLabel != "sightjack:ready" {
		t.Errorf("expected default ready label 'sightjack:ready', got %q", cfg.Labels.ReadyLabel)
	}
}

func TestLoadConfig_StrictnessOverrides(t *testing.T) {
	content := `
tracker:
  team: test
  project: test
strictness:
  default: fog
  overrides:
    security: lockdown
    performance: alert
`
	dir := t.TempDir()
	path := filepath.Join(dir, "sightjack.yaml")
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := session.LoadConfig(path)

	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if len(cfg.Strictness.Overrides) != 2 {
		t.Fatalf("expected 2 overrides, got %d", len(cfg.Strictness.Overrides))
	}
	if cfg.Strictness.Overrides["security"] != domain.StrictnessLockdown {
		t.Errorf("security: expected lockdown, got %s", cfg.Strictness.Overrides["security"])
	}
	if cfg.Strictness.Overrides["performance"] != domain.StrictnessAlert {
		t.Errorf("performance: expected alert, got %s", cfg.Strictness.Overrides["performance"])
	}
}

func TestLoadConfig_StrictnessOverrides_InvalidValueReturnsError(t *testing.T) {
	content := `
tracker:
  team: test
  project: test
strictness:
  default: fog
  overrides:
    security: nightmare
`
	dir := t.TempDir()
	path := filepath.Join(dir, "sightjack.yaml")
	os.WriteFile(path, []byte(content), 0644)

	_, err := session.LoadConfig(path)

	if err == nil {
		t.Fatal("expected error for invalid strictness override, got nil")
	}
}

func TestUpdateConfig_SetTrackerTeam(t *testing.T) {
	// given
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgPath, []byte(`
tracker:
  team: "OLD"
  project: "Test"
lang: "ja"
`), 0644)

	// when
	err := session.UpdateConfig(cfgPath, "tracker.team", "NEW")

	// then
	if err != nil {
		t.Fatalf("UpdateConfig: %v", err)
	}
	cfg, err := session.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Tracker.Team != "NEW" {
		t.Errorf("expected team 'NEW', got %q", cfg.Tracker.Team)
	}
	// unchanged fields preserved
	if cfg.Tracker.Project != "Test" {
		t.Errorf("expected project 'Test', got %q", cfg.Tracker.Project)
	}
	if cfg.Lang != "ja" {
		t.Errorf("expected lang 'ja', got %q", cfg.Lang)
	}
}

func TestUpdateConfig_SetLang(t *testing.T) {
	// given
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgPath, []byte(`
tracker:
  team: "MY"
lang: "ja"
`), 0644)

	// when
	err := session.UpdateConfig(cfgPath, "lang", "en")

	// then
	if err != nil {
		t.Fatalf("UpdateConfig: %v", err)
	}
	cfg, _ := session.LoadConfig(cfgPath)
	if cfg.Lang != "en" {
		t.Errorf("expected lang 'en', got %q", cfg.Lang)
	}
}

func TestUpdateConfig_SetStrictnessDefault(t *testing.T) {
	// given
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgPath, []byte(`
tracker:
  team: "MY"
strictness:
  default: fog
`), 0644)

	// when
	err := session.UpdateConfig(cfgPath, "strictness.default", "alert")

	// then
	if err != nil {
		t.Fatalf("UpdateConfig: %v", err)
	}
	cfg, _ := session.LoadConfig(cfgPath)
	if cfg.Strictness.Default != domain.StrictnessAlert {
		t.Errorf("expected alert, got %s", cfg.Strictness.Default)
	}
}

func TestUpdateConfig_InvalidKey_ReturnsError(t *testing.T) {
	// given
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgPath, []byte(`tracker: {team: "MY"}`), 0644)

	// when
	err := session.UpdateConfig(cfgPath, "nonexistent.key", "value")

	// then
	if err == nil {
		t.Error("expected error for invalid key")
	}
}

func TestUpdateConfig_InvalidLang_ReturnsError(t *testing.T) {
	// given
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgPath, []byte(`lang: "ja"`), 0644)

	// when
	err := session.UpdateConfig(cfgPath, "lang", "fr")

	// then
	if err == nil {
		t.Error("expected error for invalid lang value")
	}
}

func TestUpdateConfig_InvalidChunkSize_RejectsWrite(t *testing.T) {
	// given
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgPath, []byte(`scan: {chunk_size: 20}`), 0644)

	// when
	err := session.UpdateConfig(cfgPath, "scan.chunk_size", "0")

	// then
	if err == nil {
		t.Error("expected error for zero chunk_size")
	}
}

func TestUpdateConfig_InvalidStrictness_ReturnsError(t *testing.T) {
	// given
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgPath, []byte(`strictness: {default: fog}`), 0644)

	// when
	err := session.UpdateConfig(cfgPath, "strictness.default", "banana")

	// then
	if err == nil {
		t.Error("expected error for invalid strictness value")
	}
}

func TestWriteEstimatedStrictness(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "sightjack.yaml")

	initial := `
tracker:
  team: test
  project: test
strictness:
  default: fog
  overrides:
    security: lockdown
`
	os.WriteFile(cfgPath, []byte(initial), 0644)

	estimated := map[string]domain.StrictnessLevel{
		"auth-module":     domain.StrictnessAlert,
		"payment-billing": domain.StrictnessLockdown,
	}

	err := session.WriteEstimatedStrictness(cfgPath, estimated)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := session.LoadConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Strictness.Estimated["auth-module"] != domain.StrictnessAlert {
		t.Errorf("expected alert, got %s", cfg.Strictness.Estimated["auth-module"])
	}
	if cfg.Strictness.Estimated["payment-billing"] != domain.StrictnessLockdown {
		t.Errorf("expected lockdown, got %s", cfg.Strictness.Estimated["payment-billing"])
	}
	// Verify overrides preserved
	if cfg.Strictness.Overrides["security"] != domain.StrictnessLockdown {
		t.Errorf("overrides should be preserved, got %s", cfg.Strictness.Overrides["security"])
	}
}

func TestWriteEstimatedStrictness_OverwritesPrevious(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "sightjack.yaml")

	initial := `
tracker:
  team: test
  project: test
strictness:
  default: fog
  estimated:
    old-cluster: alert
`
	os.WriteFile(cfgPath, []byte(initial), 0644)

	newEstimated := map[string]domain.StrictnessLevel{
		"new-cluster": domain.StrictnessLockdown,
	}

	err := session.WriteEstimatedStrictness(cfgPath, newEstimated)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := session.LoadConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.Strictness.Estimated["old-cluster"]; ok {
		t.Error("old-cluster should be overwritten")
	}
	if cfg.Strictness.Estimated["new-cluster"] != domain.StrictnessLockdown {
		t.Errorf("expected lockdown, got %s", cfg.Strictness.Estimated["new-cluster"])
	}
}

func TestLoadConfig_EstimatedStrictnessInvalid_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sightjack.yaml")
	os.WriteFile(path, []byte(`
tracker:
  team: test
  project: test
strictness:
  default: fog
  estimated:
    bad-cluster: nightmare
`), 0644)

	_, err := session.LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for invalid estimated strictness value")
	}
}

func TestLoadConfig_ScribeSectionMissing_DefaultsToEnabled(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "sightjack.yaml")
	err := os.WriteFile(cfgPath, []byte(`
tracker:
  team: "TEST"
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := session.LoadConfig(cfgPath)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.Scribe.Enabled {
		t.Error("expected Scribe.Enabled to default to true when section missing")
	}
}
