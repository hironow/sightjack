package port

import (
	"context"
	"io"
)

// RunOption configures optional behavior for a ClaudeRunner invocation.
type RunOption func(*RunConfig)

// RunConfig holds per-invocation configuration for ClaudeRunner.
type RunConfig struct {
	AllowedTools []string
}

// ApplyOptions applies RunOption functions to a RunConfig and returns it.
func ApplyOptions(opts ...RunOption) RunConfig {
	var c RunConfig
	for _, opt := range opts {
		opt(&c)
	}
	return c
}

// WithAllowedTools restricts the tools available to the Claude model.
func WithAllowedTools(tools ...string) RunOption {
	return func(c *RunConfig) {
		c.AllowedTools = tools
	}
}

// ClaudeRunner executes the Claude CLI and returns the result text.
// Implementations may stream intermediate output to w.
type ClaudeRunner interface {
	Run(ctx context.Context, prompt string, w io.Writer, opts ...RunOption) (string, error)
}
