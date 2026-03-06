package domain_test

import (
	"testing"

	"github.com/hironow/sightjack/internal/domain"
)

func TestDefaultConfig_ScribeEnabled(t *testing.T) {
	// given/when
	cfg := domain.DefaultConfig()

	// then
	if !cfg.Scribe.Enabled {
		t.Error("expected Scribe.Enabled to be true by default")
	}
}

func TestDefaultConfig_StrictnessFog(t *testing.T) {
	// when
	cfg := domain.DefaultConfig()

	// then
	if cfg.Strictness.Default != domain.StrictnessFog {
		t.Errorf("expected fog, got %s", cfg.Strictness.Default)
	}
}

func TestDoDTemplatesInDefaultConfig(t *testing.T) {
	cfg := domain.DefaultConfig()
	if cfg.DoDTemplates != nil {
		t.Fatalf("expected nil DoDTemplates in default config, got %v", cfg.DoDTemplates)
	}
}

func TestRetryConfigDefaults(t *testing.T) {
	cfg := domain.DefaultConfig()
	if cfg.Retry.MaxAttempts != 3 {
		t.Errorf("expected MaxAttempts=3, got %d", cfg.Retry.MaxAttempts)
	}
	if cfg.Retry.BaseDelaySec != 2 {
		t.Errorf("expected BaseDelaySec=2, got %d", cfg.Retry.BaseDelaySec)
	}
}

func TestLabelsConfigDefaults(t *testing.T) {
	cfg := domain.DefaultConfig()
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
	cfg := domain.StrictnessConfig{Default: domain.StrictnessFog}

	result := domain.ResolveStrictness(cfg, []string{"feature", "bug"})

	if result != domain.StrictnessFog {
		t.Errorf("expected fog, got %s", result)
	}
}

func TestResolveStrictness_SingleLabelMatch(t *testing.T) {
	cfg := domain.StrictnessConfig{
		Default:   domain.StrictnessFog,
		Overrides: map[string]domain.StrictnessLevel{"security": domain.StrictnessLockdown},
	}

	result := domain.ResolveStrictness(cfg, []string{"feature", "security"})

	if result != domain.StrictnessLockdown {
		t.Errorf("expected lockdown, got %s", result)
	}
}

func TestResolveStrictness_StrictestWins(t *testing.T) {
	cfg := domain.StrictnessConfig{
		Default: domain.StrictnessFog,
		Overrides: map[string]domain.StrictnessLevel{
			"enhancement": domain.StrictnessAlert,
			"security":    domain.StrictnessLockdown,
		},
	}

	result := domain.ResolveStrictness(cfg, []string{"enhancement", "security"})

	if result != domain.StrictnessLockdown {
		t.Errorf("expected lockdown (strictest), got %s", result)
	}
}

func TestResolveStrictness_NilOverrides(t *testing.T) {
	cfg := domain.StrictnessConfig{Default: domain.StrictnessAlert}

	result := domain.ResolveStrictness(cfg, []string{"anything"})

	if result != domain.StrictnessAlert {
		t.Errorf("expected alert default, got %s", result)
	}
}

func TestResolveStrictness_EmptyLabels(t *testing.T) {
	cfg := domain.StrictnessConfig{
		Default:   domain.StrictnessFog,
		Overrides: map[string]domain.StrictnessLevel{"security": domain.StrictnessLockdown},
	}

	result := domain.ResolveStrictness(cfg, nil)

	if result != domain.StrictnessFog {
		t.Errorf("expected fog default, got %s", result)
	}
}

func TestResolveStrictness_NoMatchingLabels(t *testing.T) {
	cfg := domain.StrictnessConfig{
		Default:   domain.StrictnessFog,
		Overrides: map[string]domain.StrictnessLevel{"security": domain.StrictnessLockdown},
	}

	result := domain.ResolveStrictness(cfg, []string{"feature", "backend"})

	if result != domain.StrictnessFog {
		t.Errorf("expected fog default, got %s", result)
	}
}

func TestResolveStrictness_OverrideCanLowerStrictness(t *testing.T) {
	cfg := domain.StrictnessConfig{
		Default:   domain.StrictnessLockdown,
		Overrides: map[string]domain.StrictnessLevel{"Docs": domain.StrictnessFog},
	}

	result := domain.ResolveStrictness(cfg, []string{"Docs"})

	if result != domain.StrictnessFog {
		t.Errorf("expected fog override to win over lockdown default, got %s", result)
	}
}

func TestResolveStrictness_MultipleMatchesPickStrictest(t *testing.T) {
	cfg := domain.StrictnessConfig{
		Default: domain.StrictnessLockdown,
		Overrides: map[string]domain.StrictnessLevel{
			"Docs":     domain.StrictnessFog,
			"Security": domain.StrictnessAlert,
		},
	}

	result := domain.ResolveStrictness(cfg, []string{"Docs", "Security"})

	if result != domain.StrictnessAlert {
		t.Errorf("expected alert (strictest matched override), got %s", result)
	}
}

func TestResolveStrictness_ClusterNameAsLabel(t *testing.T) {
	cfg := domain.StrictnessConfig{
		Default:   domain.StrictnessFog,
		Overrides: map[string]domain.StrictnessLevel{"Security": domain.StrictnessLockdown},
	}

	result := domain.ResolveStrictness(cfg, []string{"Security"})

	if result != domain.StrictnessLockdown {
		t.Errorf("expected lockdown for Security cluster, got %s", result)
	}
}

func TestResolveStrictness_CaseInsensitiveMatch(t *testing.T) {
	cfg := domain.StrictnessConfig{
		Default:   domain.StrictnessFog,
		Overrides: map[string]domain.StrictnessLevel{"security": domain.StrictnessLockdown},
	}

	result := domain.ResolveStrictness(cfg, []string{"Security"})

	if result != domain.StrictnessLockdown {
		t.Errorf("expected lockdown (case-insensitive match), got %s", result)
	}
}

func TestValidLang_AcceptsJaAndEn(t *testing.T) {
	for _, lang := range []string{"ja", "en"} {
		if !domain.ValidLang(lang) {
			t.Errorf("expected ValidLang(%q) = true", lang)
		}
	}
}

func TestValidLang_RejectsInvalid(t *testing.T) {
	for _, lang := range []string{"jp", "EN", "english", "fr", ""} {
		if domain.ValidLang(lang) {
			t.Errorf("expected ValidLang(%q) = false", lang)
		}
	}
}
