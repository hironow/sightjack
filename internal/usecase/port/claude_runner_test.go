package port

import (
	"context"
	"errors"
	"io"
	"testing"
)

// stubRunner is a minimal ClaudeRunner for testing.
type stubRunner struct {
	calls   int
	failN   int // fail the first N calls
	output  string
	lastW   io.Writer
	lastOpt RunConfig
}

func (s *stubRunner) Run(ctx context.Context, prompt string, w io.Writer, opts ...RunOption) (string, error) {
	s.calls++
	s.lastW = w
	s.lastOpt = ApplyOptions(opts...)
	if s.calls <= s.failN {
		return "", errors.New("claude exit: non-zero")
	}
	return s.output, nil
}

func TestClaudeRunner_InterfaceSatisfied(t *testing.T) {
	// given
	var runner ClaudeRunner = &stubRunner{output: "ok"}

	// when
	result, err := runner.Run(context.Background(), "test", io.Discard)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "ok" {
		t.Errorf("expected 'ok', got %q", result)
	}
}

func TestWithAllowedTools_SetsConfig(t *testing.T) {
	// given
	opt := WithAllowedTools("Read", "Write", "mcp__linear__get_issue")

	// when
	cfg := ApplyOptions(opt)

	// then
	if len(cfg.AllowedTools) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(cfg.AllowedTools))
	}
	if cfg.AllowedTools[0] != "Read" {
		t.Errorf("expected 'Read', got %q", cfg.AllowedTools[0])
	}
}

func TestApplyOptions_Empty(t *testing.T) {
	// when
	cfg := ApplyOptions()

	// then
	if cfg.AllowedTools != nil {
		t.Errorf("expected nil AllowedTools, got %v", cfg.AllowedTools)
	}
}
