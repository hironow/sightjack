package sightjack

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// StrictnessConfig holds DoD strictness level settings.
type StrictnessConfig struct {
	Default StrictnessLevel `yaml:"default"`
}

// DoDTemplate holds must/should Definition of Done items for a category.
type DoDTemplate struct {
	Must   []string `yaml:"must"`
	Should []string `yaml:"should"`
}

// Config holds the top-level sightjack configuration loaded from YAML.
type Config struct {
	Linear     LinearConfig    `yaml:"linear"`
	Scan       ScanConfig      `yaml:"scan"`
	Claude     ClaudeConfig    `yaml:"claude"`
	Scribe     ScribeConfig    `yaml:"scribe"`
	Strictness   StrictnessConfig       `yaml:"strictness"`
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

	return &cfg, nil
}
