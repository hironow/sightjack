package domain

import (
	"strings"
	"time"
)

// StrictnessConfig holds DoD strictness level settings.
// Overrides are keyed by cluster name or Linear issue label (case-insensitive).
// Estimated holds auto-generated strictness from scan results.
// Resolution: max(default, estimated, overrides) — strictness only goes up.
type StrictnessConfig struct {
	Default   StrictnessLevel            `yaml:"default"`
	Overrides map[string]StrictnessLevel `yaml:"overrides"`
	Estimated map[string]StrictnessLevel `yaml:"estimated"`
}

// DoDTemplate holds must/should Definition of Done items for a category.
type DoDTemplate struct {
	Must   []string `yaml:"must"`
	Should []string `yaml:"should"`
}

// RetryConfig holds exponential backoff retry settings for Claude subprocess calls.
type RetryConfig struct {
	MaxAttempts  int `yaml:"max_attempts"`
	BaseDelaySec int `yaml:"base_delay_sec"`
}

// LabelsConfig holds Linear label assignment settings.
type LabelsConfig struct {
	Enabled    bool   `yaml:"enabled"`
	Prefix     string `yaml:"prefix"`
	ReadyLabel string `yaml:"ready_label"`
}

// DefaultWaitTimeout is the default D-Mail waiting phase timeout.
const DefaultWaitTimeout = 30 * time.Minute

// GateConfig holds convergence gate notification and approval settings.
type GateConfig struct {
	NotifyCmd    string        `yaml:"notify_cmd"`
	ApproveCmd   string        `yaml:"approve_cmd"`
	AutoApprove  bool          `yaml:"auto_approve"`
	ReviewCmd    string        `yaml:"review_cmd"`
	ReviewBudget int           `yaml:"review_budget"`  // max review cycles (0 = default 3)
	WaitTimeout  time.Duration `yaml:"wait_timeout"`   // D-Mail waiting phase timeout (0 = no timeout, <0 = disable waiting)
}

// IsAutoApprove reports whether the gate is configured to auto-approve.
func (g GateConfig) IsAutoApprove() bool { return g.AutoApprove }

// SetAutoApprove sets the auto-approve flag on the gate config.
func (g *GateConfig) SetAutoApprove(v bool) { g.AutoApprove = v }

// HasNotifyCmd reports whether a notification command is configured.
func (g GateConfig) HasNotifyCmd() bool { return g.NotifyCmd != "" }

// NotifyCmdString returns the notification command string.
func (g GateConfig) NotifyCmdString() string { return g.NotifyCmd }

// HasApproveCmd reports whether an approval command is configured.
func (g GateConfig) HasApproveCmd() bool { return g.ApproveCmd != "" }

// ApproveCmdString returns the approval command string.
func (g GateConfig) ApproveCmdString() string { return g.ApproveCmd }

// HasReviewCmd reports whether a review command is configured.
func (g GateConfig) HasReviewCmd() bool { return strings.TrimSpace(g.ReviewCmd) != "" }

// ReviewCmdString returns the review command string.
func (g GateConfig) ReviewCmdString() string { return g.ReviewCmd }

// EffectiveReviewBudget returns the review budget, defaulting to 3 if unset.
func (g GateConfig) EffectiveReviewBudget() int {
	if g.ReviewBudget <= 0 {
		return 3
	}
	return g.ReviewBudget
}

// Config holds the top-level sightjack configuration loaded from YAML.
type Config struct {
	Tracker      IssueTrackerConfig     `yaml:"tracker"`
	Scan         ScanConfig             `yaml:"scan"`
	Assistant    AIAssistantConfig      `yaml:"assistant"`
	Scribe       ScribeConfig           `yaml:"scribe"`
	Strictness   StrictnessConfig       `yaml:"strictness"`
	Retry        RetryConfig            `yaml:"retry"`
	Labels       LabelsConfig           `yaml:"labels"`
	Gate         GateConfig             `yaml:"gate"`
	DoDTemplates map[string]DoDTemplate `yaml:"dod_templates"`
	Lang         string                 `yaml:"lang"`
}

// ScribeConfig holds Scribe Agent settings.
type ScribeConfig struct {
	Enabled           bool `yaml:"enabled"`
	AutoDiscussRounds int  `yaml:"auto_discuss_rounds"`
}

// IssueTrackerConfig holds issue tracker integration settings.
type IssueTrackerConfig struct {
	Team    string `yaml:"team"`
	Project string `yaml:"project"`
	Cycle   string `yaml:"cycle"`
}

// ScanConfig holds scan behavior settings.
type ScanConfig struct {
	ChunkSize      int `yaml:"chunk_size"`
	MaxConcurrency int `yaml:"max_concurrency"`
}

// AIAssistantConfig holds AI assistant invocation settings.
type AIAssistantConfig struct {
	Command    string `yaml:"command"`
	Model      string `yaml:"model"`
	TimeoutSec int    `yaml:"timeout_sec"`
}

// strictnessRank returns a numeric rank for ordering: higher = stricter.
func strictnessRank(level StrictnessLevel) int {
	switch level {
	case StrictnessFog:
		return 0
	case StrictnessAlert:
		return 1
	case StrictnessLockdown:
		return 2
	default:
		return 0
	}
}

// ResolveStrictness determines the effective strictness level for a set of keys.
// Keys typically include the cluster name followed by Linear issue labels.
// Matching is case-insensitive. Resolution uses 3-layer max():
//
//	base = default
//	base = max(base, matched estimated values)
//	base = max(base, matched override values)
//
// Strictness can only go up, never down.
func ResolveStrictness(cfg StrictnessConfig, labels []string) StrictnessLevel {
	base := cfg.Default

	// Layer 1: Check estimated (auto-generated by scan)
	for _, label := range labels {
		lower := strings.ToLower(label)
		for key, level := range cfg.Estimated {
			if strings.ToLower(key) == lower {
				if strictnessRank(level) > strictnessRank(base) {
					base = level
				}
			}
		}
	}

	// Layer 2: Check overrides (manual, always wins if stronger)
	for _, label := range labels {
		lower := strings.ToLower(label)
		for key, level := range cfg.Overrides {
			if strings.ToLower(key) == lower {
				if strictnessRank(level) > strictnessRank(base) {
					base = level
				}
			}
		}
	}

	return base
}

// DefaultConfig returns a Config populated with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Scan: ScanConfig{
			ChunkSize:      20,
			MaxConcurrency: 3,
		},
		Assistant: AIAssistantConfig{},
		Scribe: ScribeConfig{
			Enabled:           true,
			AutoDiscussRounds: 2,
		},
		Strictness: StrictnessConfig{
			Default: StrictnessFog,
		},
		Retry: RetryConfig{
			MaxAttempts:  3,
			BaseDelaySec: 2,
		},
		Labels: LabelsConfig{
			Enabled:    true,
			Prefix:     "sightjack",
			ReadyLabel: "sightjack:ready",
		},
		Gate: GateConfig{
			WaitTimeout: DefaultWaitTimeout,
		},
		Lang: "ja",
	}
}

// ValidLang returns true if lang is a supported language code.
// Only "ja" and "en" are valid (used as template suffixes).
func ValidLang(lang string) bool {
	switch lang {
	case "ja", "en":
		return true
	}
	return false
}
