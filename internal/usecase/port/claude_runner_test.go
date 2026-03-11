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

func TestWithWorkDir_SetsConfig(t *testing.T) {
	// when
	cfg := ApplyOptions(WithWorkDir("/tmp/repo"))

	// then
	if cfg.WorkDir != "/tmp/repo" {
		t.Errorf("expected WorkDir '/tmp/repo', got %q", cfg.WorkDir)
	}
}

func TestWithContinue_SetsConfig(t *testing.T) {
	// when
	cfg := ApplyOptions(WithContinue())

	// then
	if !cfg.Continue {
		t.Error("expected Continue to be true")
	}
}

func TestApplyOptions_Combined(t *testing.T) {
	// when
	cfg := ApplyOptions(
		WithAllowedTools("Read"),
		WithWorkDir("/repo"),
		WithContinue(),
	)

	// then
	if len(cfg.AllowedTools) != 1 || cfg.AllowedTools[0] != "Read" {
		t.Errorf("unexpected AllowedTools: %v", cfg.AllowedTools)
	}
	if cfg.WorkDir != "/repo" {
		t.Errorf("expected WorkDir '/repo', got %q", cfg.WorkDir)
	}
	if !cfg.Continue {
		t.Error("expected Continue to be true")
	}
}
