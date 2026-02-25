package sightjack_test

import (
	"testing"

	"github.com/hironow/sightjack"
)

func TestDefaultConfig_ScribeEnabled(t *testing.T) {
	// given/when
	cfg := sightjack.DefaultConfig()

	// then
	if !cfg.Scribe.Enabled {
		t.Error("expected Scribe.Enabled to be true by default")
	}
}

func TestDefaultConfig_StrictnessFog(t *testing.T) {
	// when
	cfg := sightjack.DefaultConfig()

	// then
	if cfg.Strictness.Default != sightjack.StrictnessFog {
		t.Errorf("expected fog, got %s", cfg.Strictness.Default)
	}
}

func TestDoDTemplatesInDefaultConfig(t *testing.T) {
	cfg := sightjack.DefaultConfig()
	if cfg.DoDTemplates != nil {
		t.Fatalf("expected nil DoDTemplates in default config, got %v", cfg.DoDTemplates)
	}
}

func TestRetryConfigDefaults(t *testing.T) {
	cfg := sightjack.DefaultConfig()
	if cfg.Retry.MaxAttempts != 3 {
		t.Errorf("expected MaxAttempts=3, got %d", cfg.Retry.MaxAttempts)
	}
	if cfg.Retry.BaseDelaySec != 2 {
		t.Errorf("expected BaseDelaySec=2, got %d", cfg.Retry.BaseDelaySec)
	}
}

func TestLabelsConfigDefaults(t *testing.T) {
	cfg := sightjack.DefaultConfig()
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

func TestResolveStrictness_DefaultWhenNoOverrides(t *testing.T) {
	cfg := sightjack.StrictnessConfig{Default: sightjack.StrictnessFog}

	result := sightjack.ResolveStrictness(cfg, []string{"feature", "bug"})

	if result != sightjack.StrictnessFog {
		t.Errorf("expected fog, got %s", result)
	}
}

func TestResolveStrictness_SingleLabelMatch(t *testing.T) {
	cfg := sightjack.StrictnessConfig{
		Default:   sightjack.StrictnessFog,
		Overrides: map[string]sightjack.StrictnessLevel{"security": sightjack.StrictnessLockdown},
	}

	result := sightjack.ResolveStrictness(cfg, []string{"feature", "security"})

	if result != sightjack.StrictnessLockdown {
		t.Errorf("expected lockdown, got %s", result)
	}
}

func TestResolveStrictness_StrictestWins(t *testing.T) {
	cfg := sightjack.StrictnessConfig{
		Default: sightjack.StrictnessFog,
		Overrides: map[string]sightjack.StrictnessLevel{
			"enhancement": sightjack.StrictnessAlert,
			"security":    sightjack.StrictnessLockdown,
		},
	}

	result := sightjack.ResolveStrictness(cfg, []string{"enhancement", "security"})

	if result != sightjack.StrictnessLockdown {
		t.Errorf("expected lockdown (strictest), got %s", result)
	}
}

func TestResolveStrictness_NilOverrides(t *testing.T) {
	cfg := sightjack.StrictnessConfig{Default: sightjack.StrictnessAlert}

	result := sightjack.ResolveStrictness(cfg, []string{"anything"})

	if result != sightjack.StrictnessAlert {
		t.Errorf("expected alert default, got %s", result)
	}
}

func TestResolveStrictness_EmptyLabels(t *testing.T) {
	cfg := sightjack.StrictnessConfig{
		Default:   sightjack.StrictnessFog,
		Overrides: map[string]sightjack.StrictnessLevel{"security": sightjack.StrictnessLockdown},
	}

	result := sightjack.ResolveStrictness(cfg, nil)

	if result != sightjack.StrictnessFog {
		t.Errorf("expected fog default, got %s", result)
	}
}

func TestResolveStrictness_NoMatchingLabels(t *testing.T) {
	cfg := sightjack.StrictnessConfig{
		Default:   sightjack.StrictnessFog,
		Overrides: map[string]sightjack.StrictnessLevel{"security": sightjack.StrictnessLockdown},
	}

	result := sightjack.ResolveStrictness(cfg, []string{"feature", "backend"})

	if result != sightjack.StrictnessFog {
		t.Errorf("expected fog default, got %s", result)
	}
}

func TestResolveStrictness_OverrideCanLowerStrictness(t *testing.T) {
	cfg := sightjack.StrictnessConfig{
		Default:   sightjack.StrictnessLockdown,
		Overrides: map[string]sightjack.StrictnessLevel{"Docs": sightjack.StrictnessFog},
	}

	result := sightjack.ResolveStrictness(cfg, []string{"Docs"})

	if result != sightjack.StrictnessFog {
		t.Errorf("expected fog override to win over lockdown default, got %s", result)
	}
}

func TestResolveStrictness_MultipleMatchesPickStrictest(t *testing.T) {
	cfg := sightjack.StrictnessConfig{
		Default: sightjack.StrictnessLockdown,
		Overrides: map[string]sightjack.StrictnessLevel{
			"Docs":     sightjack.StrictnessFog,
			"Security": sightjack.StrictnessAlert,
		},
	}

	result := sightjack.ResolveStrictness(cfg, []string{"Docs", "Security"})

	if result != sightjack.StrictnessAlert {
		t.Errorf("expected alert (strictest matched override), got %s", result)
	}
}

func TestResolveStrictness_ClusterNameAsLabel(t *testing.T) {
	cfg := sightjack.StrictnessConfig{
		Default:   sightjack.StrictnessFog,
		Overrides: map[string]sightjack.StrictnessLevel{"Security": sightjack.StrictnessLockdown},
	}

	result := sightjack.ResolveStrictness(cfg, []string{"Security"})

	if result != sightjack.StrictnessLockdown {
		t.Errorf("expected lockdown for Security cluster, got %s", result)
	}
}

func TestResolveStrictness_CaseInsensitiveMatch(t *testing.T) {
	cfg := sightjack.StrictnessConfig{
		Default:   sightjack.StrictnessFog,
		Overrides: map[string]sightjack.StrictnessLevel{"security": sightjack.StrictnessLockdown},
	}

	result := sightjack.ResolveStrictness(cfg, []string{"Security"})

	if result != sightjack.StrictnessLockdown {
		t.Errorf("expected lockdown (case-insensitive match), got %s", result)
	}
}

func TestValidLang_AcceptsJaAndEn(t *testing.T) {
	for _, lang := range []string{"ja", "en"} {
		if !sightjack.ValidLang(lang) {
			t.Errorf("expected ValidLang(%q) = true", lang)
		}
	}
}

func TestValidLang_RejectsInvalid(t *testing.T) {
	for _, lang := range []string{"jp", "EN", "english", "fr", ""} {
		if sightjack.ValidLang(lang) {
			t.Errorf("expected ValidLang(%q) = false", lang)
		}
	}
}
