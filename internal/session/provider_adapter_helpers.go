package session

import "github.com/hironow/sightjack/internal/domain"

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
