package sightjack

import (
	"strings"
)

// StrictnessConfig holds DoD strictness level settings.
// Overrides are keyed by cluster name or Linear issue label (case-insensitive).
type StrictnessConfig struct {
	Default   StrictnessLevel            `yaml:"default"`
	Overrides map[string]StrictnessLevel `yaml:"overrides"`
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

// GateConfig holds convergence gate notification and approval settings.
type GateConfig struct {
	NotifyCmd   string `yaml:"notify_cmd"`
	ApproveCmd  string `yaml:"approve_cmd"`
	AutoApprove bool   `yaml:"auto_approve"`
	ReviewCmd   string `yaml:"review_cmd"`
}

// Config holds the top-level sightjack configuration loaded from YAML.
type Config struct {
	Linear       LinearConfig           `yaml:"linear"`
	Scan         ScanConfig             `yaml:"scan"`
	Claude       ClaudeConfig           `yaml:"claude"`
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
	Enabled bool `yaml:"enabled"`
}

// LinearConfig holds Linear integration settings.
type LinearConfig struct {
	Team    string `yaml:"team"`
	Project string `yaml:"project"`
	Cycle   string `yaml:"cycle"`
}

// ScanConfig holds scan behavior settings.
type ScanConfig struct {
	ChunkSize      int `yaml:"chunk_size"`
	MaxConcurrency int `yaml:"max_concurrency"`
}

// ClaudeConfig holds Claude Code invocation settings.
type ClaudeConfig struct {
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
// Matching is case-insensitive. When multiple keys match, the strictest override
// wins (lockdown > alert > fog), even if less strict than the default.
// Returns the default level only when no overrides match.
func ResolveStrictness(cfg StrictnessConfig, labels []string) StrictnessLevel {
	if len(cfg.Overrides) == 0 || len(labels) == 0 {
		return cfg.Default
	}
	matched := false
	var best StrictnessLevel
	for _, label := range labels {
		lower := strings.ToLower(label)
		for key, level := range cfg.Overrides {
			if strings.ToLower(key) == lower {
				if !matched || strictnessRank(level) > strictnessRank(best) {
					best = level
					matched = true
				}
			}
		}
	}
	if !matched {
		return cfg.Default
	}
	return best
}

// DefaultConfig returns a Config populated with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Scan: ScanConfig{
			ChunkSize:      20,
			MaxConcurrency: 3,
		},
		Claude: ClaudeConfig{
			Command:    "claude",
			Model:      "opus",
			TimeoutSec: 300,
		},
		Scribe: ScribeConfig{
			Enabled: true,
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
