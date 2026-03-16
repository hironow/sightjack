package domain

import (
	"fmt"
	"strings"
	"time"
)

// StrictnessConfig holds user-settable DoD strictness level settings.
// Overrides are keyed by cluster name or Linear issue label (case-insensitive).
// Resolution: max(default, estimated, overrides) — strictness only goes up.
type StrictnessConfig struct {
	Default   StrictnessLevel            `yaml:"default"`
	Overrides map[string]StrictnessLevel `yaml:"overrides"`
}

// ComputedConfig holds system-written fields that cannot be set via config set.
type ComputedConfig struct {
	EstimatedStrictness map[string]StrictnessLevel `yaml:"estimated_strictness,omitempty"`
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

// ApproverConfig describes how approval behavior is configured.
// Implemented by GateConfig. Used by session.BuildApprover.
type ApproverConfig interface {
	IsAutoApprove() bool
	ApproveCmdString() string
}

// GateConfig holds convergence gate notification and approval settings.
// GateConfig implements ApproverConfig.
type GateConfig struct {
	NotifyCmd    string        `yaml:"notify_cmd"`
	ApproveCmd   string        `yaml:"approve_cmd"`
	AutoApprove  bool          `yaml:"auto_approve"`
	ReviewCmd    string        `yaml:"review_cmd"`
	ReviewBudget int           `yaml:"review_budget"` // max review cycles (0 = default 3)
	WaitTimeout  time.Duration `yaml:"wait_timeout"`  // D-Mail waiting phase timeout (0 = 24h safety cap, <0 = disable waiting)
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

// SetNotifyCmd sets the notification command.
func (g *GateConfig) SetNotifyCmd(cmd string) { g.NotifyCmd = cmd }

// SetApproveCmd sets the approval command.
func (g *GateConfig) SetApproveCmd(cmd string) { g.ApproveCmd = cmd }

// SetReviewCmd sets the review command.
func (g *GateConfig) SetReviewCmd(cmd string) { g.ReviewCmd = cmd }

// SetReviewBudget sets the max review cycles.
func (g *GateConfig) SetReviewBudget(n int) { g.ReviewBudget = n }

// SetWaitTimeout sets the D-Mail waiting phase timeout.
func (g *GateConfig) SetWaitTimeout(d time.Duration) { g.WaitTimeout = d }

// Default values for Config fields. Used by DefaultConfig and post-load
// validation to avoid hardcoded strings throughout the codebase.
const (
	DefaultClaudeCmd  = "claude"
	DefaultModel      = "opus"
	DefaultTimeoutSec = 1980
)

// Config holds the top-level sightjack configuration loaded from YAML.
type Config struct {
	Tracker      IssueTrackerConfig     `yaml:"tracker"`
	Scan         ScanConfig             `yaml:"scan"`
	ClaudeCmd    string                 `yaml:"claude_cmd,omitempty"`
	Model        string                 `yaml:"model,omitempty"`
	TimeoutSec   int                    `yaml:"timeout_sec,omitempty"`
	Scribe       ScribeConfig           `yaml:"scribe"`
	Strictness   StrictnessConfig       `yaml:"strictness"`
	Retry        RetryConfig            `yaml:"retry"`
	Labels       LabelsConfig           `yaml:"labels"`
	Gate         GateConfig             `yaml:"gate"`
	DoDTemplates map[string]DoDTemplate `yaml:"dod_templates"`
	Lang         string                 `yaml:"lang"`
	Computed     ComputedConfig         `yaml:"computed,omitempty"`
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
// The estimated parameter comes from ComputedConfig.EstimatedStrictness.
func ResolveStrictness(cfg StrictnessConfig, estimated map[string]StrictnessLevel, labels []string) StrictnessLevel {
	base := cfg.Default

	// Layer 1: Check estimated (auto-generated by scan)
	for _, label := range labels {
		lower := strings.ToLower(label)
		for key, level := range estimated {
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
		ClaudeCmd:  DefaultClaudeCmd,
		Model:      DefaultModel,
		TimeoutSec: DefaultTimeoutSec,
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

// ValidateConfig checks the config for consistency and returns a list of errors.
// An empty slice means the config is valid.
func ValidateConfig(cfg Config) []string {
	var errs []string

	if cfg.ClaudeCmd == "" {
		errs = append(errs, "claude_cmd must not be empty")
	}
	if cfg.Model == "" {
		errs = append(errs, "model must not be empty")
	}

	if cfg.Lang != "" && !ValidLang(cfg.Lang) {
		errs = append(errs, fmt.Sprintf("lang must be \"ja\" or \"en\" (got %q)", cfg.Lang))
	}
	if !cfg.Strictness.Default.Valid() {
		errs = append(errs, fmt.Sprintf("strictness.default must be fog, alert, or lockdown (got %q)", cfg.Strictness.Default))
	}
	if cfg.Scan.ChunkSize < 1 {
		errs = append(errs, fmt.Sprintf("scan.chunk_size must be positive (got %d)", cfg.Scan.ChunkSize))
	}
	if cfg.Scan.MaxConcurrency < 1 {
		errs = append(errs, fmt.Sprintf("scan.max_concurrency must be positive (got %d)", cfg.Scan.MaxConcurrency))
	}
	if cfg.TimeoutSec < 0 {
		errs = append(errs, fmt.Sprintf("timeout_sec must be non-negative (got %d)", cfg.TimeoutSec))
	}
	if cfg.Retry.MaxAttempts < 1 {
		errs = append(errs, fmt.Sprintf("retry.max_attempts must be positive (got %d)", cfg.Retry.MaxAttempts))
	}
	if cfg.Retry.BaseDelaySec < 1 {
		errs = append(errs, fmt.Sprintf("retry.base_delay_sec must be positive (got %d)", cfg.Retry.BaseDelaySec))
	}
	for label, level := range cfg.Strictness.Overrides {
		if !level.Valid() {
			errs = append(errs, fmt.Sprintf("strictness.overrides[%q] is invalid: %q", label, level))
		}
	}
	for label, level := range cfg.Computed.EstimatedStrictness {
		if !level.Valid() {
			errs = append(errs, fmt.Sprintf("computed.estimated_strictness[%q] is invalid: %q", label, level))
		}
	}

	return errs
}
