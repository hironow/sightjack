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

func TestRetryConfigDefaults(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Retry.MaxAttempts != 3 {
		t.Errorf("expected MaxAttempts=3, got %d", cfg.Retry.MaxAttempts)
	}
	if cfg.Retry.BaseDelaySec != 2 {
		t.Errorf("expected BaseDelaySec=2, got %d", cfg.Retry.BaseDelaySec)
	}
}

func TestLoadConfigWithRetry(t *testing.T) {
	content := `
linear:
  team: test
  project: test
retry:
  max_attempts: 5
  base_delay_sec: 1
`
	dir := t.TempDir()
	path := filepath.Join(dir, "sightjack.yaml")
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := LoadConfig(path)
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
linear:
  team: test
  project: test
retry:
  max_attempts: 0
  base_delay_sec: -1
`
	dir := t.TempDir()
	path := filepath.Join(dir, "sightjack.yaml")
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := LoadConfig(path)
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

func TestLabelsConfigDefaults(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.Labels.Enabled {
		t.Error("expected Labels.Enabled=true by default")
	}
	if cfg.Labels.Prefix != "sightjack" {
		t.Errorf("expected Prefix='sightjack', got %q", cfg.Labels.Prefix)
	}
	if cfg.Labels.ReadyLabel != "sightjack:ready" {
		t.Errorf("expected ReadyLabel='sightjack:ready', got %q", cfg.Labels.ReadyLabel)
	}
}

func TestLoadConfigWithLabels(t *testing.T) {
	content := `
linear:
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

	cfg, err := LoadConfig(path)
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
	// given: labels enabled with explicitly empty prefix and ready_label
	content := `
linear:
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

	// when
	cfg, err := LoadConfig(path)

	// then
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

func TestResolveStrictness_DefaultWhenNoOverrides(t *testing.T) {
	// given: config with no overrides
	cfg := StrictnessConfig{Default: StrictnessFog}

	// when
	result := ResolveStrictness(cfg, []string{"feature", "bug"})

	// then
	if result != StrictnessFog {
		t.Errorf("expected fog, got %s", result)
	}
}

func TestResolveStrictness_SingleLabelMatch(t *testing.T) {
	// given: override for "security" label
	cfg := StrictnessConfig{
		Default:   StrictnessFog,
		Overrides: map[string]StrictnessLevel{"security": StrictnessLockdown},
	}

	// when
	result := ResolveStrictness(cfg, []string{"feature", "security"})

	// then
	if result != StrictnessLockdown {
		t.Errorf("expected lockdown, got %s", result)
	}
}

func TestResolveStrictness_StrictestWins(t *testing.T) {
	// given: multiple matching overrides with different levels
	cfg := StrictnessConfig{
		Default: StrictnessFog,
		Overrides: map[string]StrictnessLevel{
			"enhancement": StrictnessAlert,
			"security":    StrictnessLockdown,
		},
	}

	// when: both labels match
	result := ResolveStrictness(cfg, []string{"enhancement", "security"})

	// then: lockdown > alert, so lockdown wins
	if result != StrictnessLockdown {
		t.Errorf("expected lockdown (strictest), got %s", result)
	}
}

func TestResolveStrictness_NilOverrides(t *testing.T) {
	// given: nil overrides map
	cfg := StrictnessConfig{Default: StrictnessAlert}

	// when
	result := ResolveStrictness(cfg, []string{"anything"})

	// then
	if result != StrictnessAlert {
		t.Errorf("expected alert default, got %s", result)
	}
}

func TestResolveStrictness_EmptyLabels(t *testing.T) {
	// given: overrides exist but no labels provided
	cfg := StrictnessConfig{
		Default:   StrictnessFog,
		Overrides: map[string]StrictnessLevel{"security": StrictnessLockdown},
	}

	// when
	result := ResolveStrictness(cfg, nil)

	// then
	if result != StrictnessFog {
		t.Errorf("expected fog default, got %s", result)
	}
}

func TestResolveStrictness_NoMatchingLabels(t *testing.T) {
	// given: overrides exist but labels don't match
	cfg := StrictnessConfig{
		Default:   StrictnessFog,
		Overrides: map[string]StrictnessLevel{"security": StrictnessLockdown},
	}

	// when
	result := ResolveStrictness(cfg, []string{"feature", "backend"})

	// then
	if result != StrictnessFog {
		t.Errorf("expected fog default, got %s", result)
	}
}

func TestResolveStrictness_OverrideCanLowerStrictness(t *testing.T) {
	// given: default is lockdown, but override lowers "Docs" to fog
	cfg := StrictnessConfig{
		Default:   StrictnessLockdown,
		Overrides: map[string]StrictnessLevel{"Docs": StrictnessFog},
	}

	// when: label matches the lower override
	result := ResolveStrictness(cfg, []string{"Docs"})

	// then: override wins even though it's less strict than default
	if result != StrictnessFog {
		t.Errorf("expected fog override to win over lockdown default, got %s", result)
	}
}

func TestResolveStrictness_MultipleMatchesPickStrictest(t *testing.T) {
	// given: default lockdown, two matching overrides at different levels
	cfg := StrictnessConfig{
		Default: StrictnessLockdown,
		Overrides: map[string]StrictnessLevel{
			"Docs":     StrictnessFog,
			"Security": StrictnessAlert,
		},
	}

	// when: both labels match
	result := ResolveStrictness(cfg, []string{"Docs", "Security"})

	// then: strictest among matched overrides (alert > fog), not default
	if result != StrictnessAlert {
		t.Errorf("expected alert (strictest matched override), got %s", result)
	}
}

func TestResolveStrictness_ClusterNameAsLabel(t *testing.T) {
	// given: overrides keyed by cluster name
	cfg := StrictnessConfig{
		Default:   StrictnessFog,
		Overrides: map[string]StrictnessLevel{"Security": StrictnessLockdown},
	}

	// when: cluster name passed as label
	result := ResolveStrictness(cfg, []string{"Security"})

	// then: override should match the cluster name
	if result != StrictnessLockdown {
		t.Errorf("expected lockdown for Security cluster, got %s", result)
	}
}

func TestResolveStrictness_CaseInsensitiveMatch(t *testing.T) {
	// given: override key in lowercase, label in mixed case
	cfg := StrictnessConfig{
		Default:   StrictnessFog,
		Overrides: map[string]StrictnessLevel{"security": StrictnessLockdown},
	}

	// when: label has different casing
	result := ResolveStrictness(cfg, []string{"Security"})

	// then: should match case-insensitively
	if result != StrictnessLockdown {
		t.Errorf("expected lockdown (case-insensitive match), got %s", result)
	}
}

func TestLoadConfig_StrictnessOverrides(t *testing.T) {
	// given: config with strictness overrides
	content := `
linear:
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

	// when
	cfg, err := LoadConfig(path)

	// then
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if len(cfg.Strictness.Overrides) != 2 {
		t.Fatalf("expected 2 overrides, got %d", len(cfg.Strictness.Overrides))
	}
	if cfg.Strictness.Overrides["security"] != StrictnessLockdown {
		t.Errorf("security: expected lockdown, got %s", cfg.Strictness.Overrides["security"])
	}
	if cfg.Strictness.Overrides["performance"] != StrictnessAlert {
		t.Errorf("performance: expected alert, got %s", cfg.Strictness.Overrides["performance"])
	}
}

func TestLoadConfig_StrictnessOverrides_InvalidValueReturnsError(t *testing.T) {
	// given: config with an invalid strictness override value
	content := `
linear:
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

	// when
	_, err := LoadConfig(path)

	// then: should return an error for the invalid override value
	if err == nil {
		t.Fatal("expected error for invalid strictness override, got nil")
	}
}

func TestValidLang_AcceptsJaAndEn(t *testing.T) {
	for _, lang := range []string{"ja", "en"} {
		if !ValidLang(lang) {
			t.Errorf("expected ValidLang(%q) = true", lang)
		}
	}
}

func TestValidLang_RejectsInvalid(t *testing.T) {
	for _, lang := range []string{"jp", "EN", "english", "fr", ""} {
		if ValidLang(lang) {
			t.Errorf("expected ValidLang(%q) = false", lang)
		}
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
