package session

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/hironow/sightjack/internal/domain"
)

// UpdateConfig reads a config file, updates a single key, validates, and writes back.
// Supported keys: tracker.team, tracker.project, tracker.cycle, lang, strictness.default,
// scan.chunk_size, scan.max_concurrency, assistant.model, assistant.timeout_sec,
// gate.auto_approve, labels.enabled, labels.prefix, labels.ready_label.
func UpdateConfig(path string, key string, value string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	cfg := domain.DefaultConfig()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	if err := setConfigField(&cfg, key, value); err != nil {
		return err
	}

	// Validate before writing
	if errs := domain.ValidateConfig(cfg); len(errs) > 0 {
		return fmt.Errorf("invalid config after update: %s", errs[0])
	}

	out, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return os.WriteFile(path, out, 0644)
}

func setConfigField(cfg *domain.Config, key string, value string) error {
	switch key {
	case "tracker.team":
		cfg.Tracker.Team = value
	case "tracker.project":
		cfg.Tracker.Project = value
	case "tracker.cycle":
		cfg.Tracker.Cycle = value
	case "lang":
		if !domain.ValidLang(value) {
			return fmt.Errorf("invalid lang %q: must be ja or en", value)
		}
		cfg.Lang = value
	case "strictness.default":
		level := domain.StrictnessLevel(value)
		if !level.Valid() {
			return fmt.Errorf("invalid strictness %q: must be fog, alert, or lockdown", value)
		}
		cfg.Strictness.Default = level
	case "scan.chunk_size":
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 {
			return fmt.Errorf("invalid chunk_size %q: must be positive integer", value)
		}
		cfg.Scan.ChunkSize = n
	case "scan.max_concurrency":
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 {
			return fmt.Errorf("invalid max_concurrency %q: must be positive integer", value)
		}
		cfg.Scan.MaxConcurrency = n
	case "assistant.model":
		cfg.Assistant.Model = value
	case "assistant.timeout_sec":
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 {
			return fmt.Errorf("invalid timeout_sec %q: must be positive integer", value)
		}
		cfg.Assistant.TimeoutSec = n
	case "gate.auto_approve":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid auto_approve %q: must be true or false", value)
		}
		cfg.Gate.SetAutoApprove(b)
	case "labels.enabled":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid labels.enabled %q: must be true or false", value)
		}
		cfg.Labels.Enabled = b
	case "labels.prefix":
		cfg.Labels.Prefix = value
	case "labels.ready_label":
		cfg.Labels.ReadyLabel = value
	case "assistant.command":
		cfg.Assistant.Command = value
	case "scribe.enabled":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid scribe.enabled %q: must be true or false", value)
		}
		cfg.Scribe.Enabled = b
	case "scribe.auto_discuss_rounds":
		n, err := strconv.Atoi(value)
		if err != nil || n < 0 {
			return fmt.Errorf("invalid scribe.auto_discuss_rounds %q: must be non-negative integer", value)
		}
		cfg.Scribe.AutoDiscussRounds = n
	case "retry.max_attempts":
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 {
			return fmt.Errorf("invalid retry.max_attempts %q: must be positive integer", value)
		}
		cfg.Retry.MaxAttempts = n
	case "retry.base_delay_sec":
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 {
			return fmt.Errorf("invalid retry.base_delay_sec %q: must be positive integer", value)
		}
		cfg.Retry.BaseDelaySec = n
	case "gate.notify_cmd":
		cfg.Gate.SetNotifyCmd(value)
	case "gate.approve_cmd":
		cfg.Gate.SetApproveCmd(value)
	case "gate.review_cmd":
		cfg.Gate.SetReviewCmd(value)
	case "gate.review_budget":
		n, err := strconv.Atoi(value)
		if err != nil || n < 0 {
			return fmt.Errorf("invalid gate.review_budget %q: must be non-negative integer", value)
		}
		cfg.Gate.SetReviewBudget(n)
	case "gate.wait_timeout":
		d, err := time.ParseDuration(value)
		if err != nil {
			return fmt.Errorf("invalid gate.wait_timeout %q: must be duration (e.g. 30m, 1h)", value)
		}
		cfg.Gate.SetWaitTimeout(d)
	default:
		return fmt.Errorf("unknown config key %q", key)
	}
	return nil
}

// WriteEstimatedStrictness reads the config, replaces the estimated strictness map,
// and writes back. Called after scan to persist LLM-estimated values.
func WriteEstimatedStrictness(path string, estimated map[string]domain.StrictnessLevel) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config for estimated strictness: %w", err)
	}

	cfg := domain.DefaultConfig()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse config for estimated strictness: %w", err)
	}

	cfg.Strictness.Estimated = estimated

	out, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return os.WriteFile(path, out, 0644)
}

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
		cfg.Scan.MaxConcurrency = 3
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
	if !cfg.Strictness.Default.Valid() { // nosemgrep: lod-excessive-dot-chain [permanent]
		cfg.Strictness.Default = domain.StrictnessFog
	}
	for label, level := range cfg.Strictness.Overrides {
		if !level.Valid() {
			return nil, fmt.Errorf("invalid strictness override for %q: %q", label, level)
		}
	}
	for label, level := range cfg.Strictness.Estimated {
		if !level.Valid() {
			return nil, fmt.Errorf("invalid estimated strictness for %q: %q", label, level)
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
