package session

import "github.com/hironow/sightjack/internal/domain"

// ProviderAdapterConfig holds the class-wide configuration for creating a
// provider adapter. All AI coding tools accept this shape in NewTrackedRunner.
// Role-specific policies (retry, lazy singleton) are separate from this contract.
type ProviderAdapterConfig struct {
	Cmd        string // provider CLI command (e.g. "claude")
	Model      string // model name (e.g. "opus")
	TimeoutSec int    // per-invocation timeout (0 = context deadline only)
	BaseDir    string // repository root (state dir parent)
	ToolName   string // tool identifier for stream events
}

// RetryConfig holds retry policy for RetryRunner.
// Sightjack-specific: other tools manage retry at expedition/check-cycle level.
type RetryConfig struct {
	MaxAttempts  int
	BaseDelaySec int
	TimeoutSec   int
}

// AdapterConfigFromDomainConfig extracts ProviderAdapterConfig from a domain.Config.
func AdapterConfigFromDomainConfig(cfg *domain.Config, baseDir string) ProviderAdapterConfig {
	return ProviderAdapterConfig{
		Cmd:        cfg.ClaudeCmd,
		Model:      cfg.Model,
		TimeoutSec: cfg.TimeoutSec,
		BaseDir:    baseDir,
		ToolName:   "sightjack",
	}
}

// RetryConfigFromDomainConfig extracts RetryConfig from a domain.Config.
func RetryConfigFromDomainConfig(cfg *domain.Config) RetryConfig {
	return RetryConfig{
		MaxAttempts:  cfg.Retry.MaxAttempts,
		BaseDelaySec: cfg.Retry.BaseDelaySec,
		TimeoutSec:   cfg.TimeoutSec,
	}
}
