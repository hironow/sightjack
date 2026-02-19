package sightjack

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
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

// Config holds the top-level sightjack configuration loaded from YAML.
type Config struct {
	Linear       LinearConfig           `yaml:"linear"`
	Scan         ScanConfig             `yaml:"scan"`
	Claude       ClaudeConfig           `yaml:"claude"`
	Scribe       ScribeConfig           `yaml:"scribe"`
	Strictness   StrictnessConfig       `yaml:"strictness"`
	Retry        RetryConfig            `yaml:"retry"`
	Labels       LabelsConfig           `yaml:"labels"`
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

// LoadConfig reads a YAML config file and returns a Config with defaults
// applied for any fields not specified in the file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if cfg.Scan.ChunkSize < 1 {
		cfg.Scan.ChunkSize = 20
	}
	if cfg.Scan.MaxConcurrency < 1 {
		cfg.Scan.MaxConcurrency = 1
	}
	if cfg.Claude.TimeoutSec < 1 {
		cfg.Claude.TimeoutSec = 300
	}
	if !cfg.Strictness.Default.Valid() {
		cfg.Strictness.Default = StrictnessFog
	}
	for label, level := range cfg.Strictness.Overrides {
		if !level.Valid() {
			return nil, fmt.Errorf("invalid strictness override for %q: %q", label, level)
		}
	}
	if cfg.Retry.MaxAttempts < 1 {
		cfg.Retry.MaxAttempts = 3
	}
	if cfg.Retry.BaseDelaySec < 1 {
		cfg.Retry.BaseDelaySec = 2
	}
	if cfg.Labels.Enabled {
		defCfg := DefaultConfig()
		if cfg.Labels.Prefix == "" {
			cfg.Labels.Prefix = defCfg.Labels.Prefix
		}
		if cfg.Labels.ReadyLabel == "" {
			cfg.Labels.ReadyLabel = defCfg.Labels.ReadyLabel
		}
	}

	return &cfg, nil
}
