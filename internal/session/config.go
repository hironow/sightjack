package session

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/hironow/sightjack/internal/domain"
)

// LoadConfig reads a YAML config file and returns a Config with defaults
// applied for any fields not specified in the file.
func LoadConfig(path string) (*domain.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := domain.DefaultConfig()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if cfg.Scan.ChunkSize < 1 {
		cfg.Scan.ChunkSize = 20
	}
	if cfg.Scan.MaxConcurrency < 1 {
		cfg.Scan.MaxConcurrency = 1
	}
	if cfg.Assistant.Command == "" {
		cfg.Assistant.Command = "claude"
	}
	if cfg.Assistant.Model == "" {
		cfg.Assistant.Model = "opus"
	}
	if cfg.Assistant.TimeoutSec < 1 {
		cfg.Assistant.TimeoutSec = 300
	}
	if !cfg.Strictness.Default.Valid() { // nosemgrep: lod-excessive-dot-chain
		cfg.Strictness.Default = domain.StrictnessFog
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
		defCfg := domain.DefaultConfig()
		if cfg.Labels.Prefix == "" {
			cfg.Labels.Prefix = defCfg.Labels.Prefix
		}
		if cfg.Labels.ReadyLabel == "" {
			cfg.Labels.ReadyLabel = defCfg.Labels.ReadyLabel
		}
	}

	return &cfg, nil
}
