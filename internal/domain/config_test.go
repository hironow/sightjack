package domain_test

import (
	"testing"
	"time"

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

func TestDefaultConfig_AllFields(t *testing.T) {
	// given/when
	cfg := domain.DefaultConfig()

	// then: Scan defaults
	if cfg.Scan.ChunkSize != 20 {
		t.Errorf("Scan.ChunkSize: expected 20, got %d", cfg.Scan.ChunkSize)
	}
	if cfg.Scan.MaxConcurrency != 3 {
		t.Errorf("Scan.MaxConcurrency: expected 3, got %d", cfg.Scan.MaxConcurrency)
	}

	// then: Assistant defaults (zero values before LoadConfig clamping)
	if cfg.Assistant.Command != "" {
		t.Errorf("Assistant.Command: expected empty (set by LoadConfig), got %q", cfg.Assistant.Command)
	}
	if cfg.Assistant.Model != "" {
		t.Errorf("Assistant.Model: expected empty (set by LoadConfig), got %q", cfg.Assistant.Model)
	}
	if cfg.Assistant.TimeoutSec != 0 {
		t.Errorf("Assistant.TimeoutSec: expected 0 (set by LoadConfig), got %d", cfg.Assistant.TimeoutSec)
	}

	// then: Scribe defaults
	if !cfg.Scribe.Enabled {
		t.Error("Scribe.Enabled: expected true")
	}

	// then: Strictness defaults
	if cfg.Strictness.Default != domain.StrictnessFog {
		t.Errorf("Strictness.Default: expected fog, got %s", cfg.Strictness.Default)
	}
	if cfg.Strictness.Overrides != nil {
		t.Errorf("Strictness.Overrides: expected nil, got %v", cfg.Strictness.Overrides)
	}
	if cfg.Strictness.Estimated != nil {
		t.Errorf("Strictness.Estimated: expected nil, got %v", cfg.Strictness.Estimated)
	}

	// then: Retry defaults
	if cfg.Retry.MaxAttempts != 3 {
		t.Errorf("Retry.MaxAttempts: expected 3, got %d", cfg.Retry.MaxAttempts)
	}
	if cfg.Retry.BaseDelaySec != 2 {
		t.Errorf("Retry.BaseDelaySec: expected 2, got %d", cfg.Retry.BaseDelaySec)
	}

	// then: Labels defaults
	if !cfg.Labels.Enabled {
		t.Error("Labels.Enabled: expected true")
	}
	if cfg.Labels.Prefix != "sightjack" {
		t.Errorf("Labels.Prefix: expected 'sightjack', got %q", cfg.Labels.Prefix)
	}
	if cfg.Labels.ReadyLabel != "sightjack:ready" {
		t.Errorf("Labels.ReadyLabel: expected 'sightjack:ready', got %q", cfg.Labels.ReadyLabel)
	}

	// then: Gate defaults (all zero values)
	if cfg.Gate.AutoApprove {
		t.Error("Gate.AutoApprove: expected false")
	}
	if cfg.Gate.NotifyCmd != "" {
		t.Errorf("Gate.NotifyCmd: expected empty, got %q", cfg.Gate.NotifyCmd)
	}
	if cfg.Gate.ApproveCmd != "" {
		t.Errorf("Gate.ApproveCmd: expected empty, got %q", cfg.Gate.ApproveCmd)
	}
	if cfg.Gate.ReviewCmd != "" {
		t.Errorf("Gate.ReviewCmd: expected empty, got %q", cfg.Gate.ReviewCmd)
	}
	if cfg.Gate.ReviewBudget != 0 {
		t.Errorf("Gate.ReviewBudget: expected 0, got %d", cfg.Gate.ReviewBudget)
	}
	if cfg.Gate.WaitTimeout != 30*time.Minute {
		t.Errorf("Gate.WaitTimeout: expected 30m, got %v", cfg.Gate.WaitTimeout)
	}

	// then: Tracker defaults (all zero values)
	if cfg.Tracker.Team != "" {
		t.Errorf("Tracker.Team: expected empty, got %q", cfg.Tracker.Team)
	}
	if cfg.Tracker.Project != "" {
		t.Errorf("Tracker.Project: expected empty, got %q", cfg.Tracker.Project)
	}
	if cfg.Tracker.Cycle != "" {
		t.Errorf("Tracker.Cycle: expected empty, got %q", cfg.Tracker.Cycle)
	}

	// then: DoDTemplates defaults
	if cfg.DoDTemplates != nil {
		t.Errorf("DoDTemplates: expected nil, got %v", cfg.DoDTemplates)
	}

	// then: Lang default
	if cfg.Lang != "ja" {
		t.Errorf("Lang: expected 'ja', got %q", cfg.Lang)
	}
}

func TestGateConfig_EffectiveReviewBudget(t *testing.T) {
	tests := []struct {
		name   string
		budget int
		want   int
	}{
		{"zero defaults to 3", 0, 3},
		{"negative defaults to 3", -1, 3},
		{"explicit value preserved", 5, 5},
		{"one preserved", 1, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := domain.GateConfig{ReviewBudget: tt.budget}
			if got := g.EffectiveReviewBudget(); got != tt.want {
				t.Errorf("EffectiveReviewBudget() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestGateConfig_HasMethods(t *testing.T) {
	// given
	empty := domain.GateConfig{}
	full := domain.GateConfig{
		NotifyCmd:   "echo notify",
		ApproveCmd:  "echo approve",
		AutoApprove: true,
		ReviewCmd:   "echo review",
	}

	// then: empty gate
	if empty.IsAutoApprove() {
		t.Error("empty: IsAutoApprove should be false")
	}
	if empty.HasNotifyCmd() {
		t.Error("empty: HasNotifyCmd should be false")
	}
	if empty.HasApproveCmd() {
		t.Error("empty: HasApproveCmd should be false")
	}
	if empty.HasReviewCmd() {
		t.Error("empty: HasReviewCmd should be false")
	}

	// then: full gate
	if !full.IsAutoApprove() {
		t.Error("full: IsAutoApprove should be true")
	}
	if !full.HasNotifyCmd() {
		t.Error("full: HasNotifyCmd should be true")
	}
	if full.NotifyCmdString() != "echo notify" {
		t.Errorf("full: NotifyCmdString = %q", full.NotifyCmdString())
	}
	if !full.HasApproveCmd() {
		t.Error("full: HasApproveCmd should be true")
	}
	if full.ApproveCmdString() != "echo approve" {
		t.Errorf("full: ApproveCmdString = %q", full.ApproveCmdString())
	}
	if !full.HasReviewCmd() {
		t.Error("full: HasReviewCmd should be true")
	}
	if full.ReviewCmdString() != "echo review" {
		t.Errorf("full: ReviewCmdString = %q", full.ReviewCmdString())
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

func TestResolveStrictness_OverrideCannotLowerBelowDefault(t *testing.T) {
	cfg := domain.StrictnessConfig{
		Default:   domain.StrictnessLockdown,
		Overrides: map[string]domain.StrictnessLevel{"Docs": domain.StrictnessFog},
	}

	result := domain.ResolveStrictness(cfg, []string{"Docs"})

	if result != domain.StrictnessLockdown {
		t.Errorf("expected lockdown (max never lowers), got %s", result)
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

	if result != domain.StrictnessLockdown {
		t.Errorf("expected lockdown (default is strictest via max), got %s", result)
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

func TestDefaultWaitTimeout(t *testing.T) {
	if domain.DefaultWaitTimeout != 30*time.Minute {
		t.Errorf("got %v, want 30m", domain.DefaultWaitTimeout)
	}
}

func TestDefaultConfig_WaitTimeout(t *testing.T) {
	// given/when
	cfg := domain.DefaultConfig()

	// then
	if cfg.Gate.WaitTimeout != 30*time.Minute {
		t.Errorf("Gate.WaitTimeout: expected 30m, got %v", cfg.Gate.WaitTimeout)
	}
}

func TestResolveStrictness_EstimatedTakesEffect(t *testing.T) {
	// given
	cfg := domain.StrictnessConfig{
		Default:   domain.StrictnessFog,
		Estimated: map[string]domain.StrictnessLevel{"auth-module": domain.StrictnessAlert},
	}

	// when
	got := domain.ResolveStrictness(cfg, []string{"auth-module"})

	// then
	if got != domain.StrictnessAlert {
		t.Errorf("expected alert, got %s", got)
	}
}

func TestResolveStrictness_OverrideTrumpsEstimated(t *testing.T) {
	// given
	cfg := domain.StrictnessConfig{
		Default:   domain.StrictnessFog,
		Overrides: map[string]domain.StrictnessLevel{"auth-module": domain.StrictnessLockdown},
		Estimated: map[string]domain.StrictnessLevel{"auth-module": domain.StrictnessAlert},
	}

	// when
	got := domain.ResolveStrictness(cfg, []string{"auth-module"})

	// then
	if got != domain.StrictnessLockdown {
		t.Errorf("expected lockdown, got %s", got)
	}
}

func TestResolveStrictness_MaxOfDefaultAndEstimated(t *testing.T) {
	// given
	cfg := domain.StrictnessConfig{
		Default:   domain.StrictnessAlert,
		Estimated: map[string]domain.StrictnessLevel{"auth-module": domain.StrictnessFog},
	}

	// when
	got := domain.ResolveStrictness(cfg, []string{"auth-module"})

	// then
	if got != domain.StrictnessAlert {
		t.Errorf("expected alert, got %s", got)
	}
}
