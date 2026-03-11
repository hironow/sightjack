package session_test

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/platform"
	"github.com/hironow/sightjack/internal/session"
	"github.com/hironow/sightjack/internal/usecase/port"
)

// fakeRunner is a test double for ClaudeRunner.
type fakeRunner struct {
	calls    int
	failN    int
	output   string
	lastOpts port.RunConfig
}

func (f *fakeRunner) Run(ctx context.Context, prompt string, w io.Writer, opts ...port.RunOption) (string, error) {
	f.calls++
	f.lastOpts = port.ApplyOptions(opts...)
	if f.calls <= f.failN {
		return "", errors.New("claude exit: non-zero")
	}
	return f.output, nil
}

func TestRetryRunner_SucceedsFirstAttempt(t *testing.T) {
	// given
	inner := &fakeRunner{output: "ok"}
	runner := &session.RetryRunner{
		Inner:  inner,
		Retry:  domain.RetryConfig{MaxAttempts: 3, BaseDelaySec: 0},
		Logger: platform.NewLogger(io.Discard, false),
	}

	// when
	result, err := runner.Run(context.Background(), "test", io.Discard)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "ok" {
		t.Errorf("expected 'ok', got %q", result)
	}
	if inner.calls != 1 {
		t.Errorf("expected 1 call, got %d", inner.calls)
	}
}

func TestRetryRunner_RetriesAndSucceeds(t *testing.T) {
	// given: fails 2 times, succeeds on 3rd
	inner := &fakeRunner{failN: 2, output: "success"}
	runner := &session.RetryRunner{
		Inner:  inner,
		Retry:  domain.RetryConfig{MaxAttempts: 3, BaseDelaySec: 0},
		Logger: platform.NewLogger(io.Discard, false),
	}

	// when
	result, err := runner.Run(context.Background(), "test", io.Discard)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "success" {
		t.Errorf("expected 'success', got %q", result)
	}
	if inner.calls != 3 {
		t.Errorf("expected 3 calls, got %d", inner.calls)
	}
}

func TestRetryRunner_ExhaustsRetries(t *testing.T) {
	// given: always fails
	inner := &fakeRunner{failN: 100, output: "never"}
	runner := &session.RetryRunner{
		Inner:  inner,
		Retry:  domain.RetryConfig{MaxAttempts: 2, BaseDelaySec: 0},
		Logger: platform.NewLogger(io.Discard, false),
	}

	// when
	_, err := runner.Run(context.Background(), "test", io.Discard)

	// then
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if inner.calls != 2 {
		t.Errorf("expected 2 calls, got %d", inner.calls)
	}
}

func TestRetryRunner_NoRetryOnCancel(t *testing.T) {
	// given: cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	inner := &fakeRunner{failN: 100}
	runner := &session.RetryRunner{
		Inner:  inner,
		Retry:  domain.RetryConfig{MaxAttempts: 3, BaseDelaySec: 0},
		Logger: platform.NewLogger(io.Discard, false),
	}

	// when
	_, err := runner.Run(ctx, "test", io.Discard)

	// then
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
	if inner.calls > 1 {
		t.Errorf("expected at most 1 call on cancel, got %d", inner.calls)
	}
}

func TestRetryRunner_ForwardsOptions(t *testing.T) {
	// given
	inner := &fakeRunner{output: "ok"}
	runner := &session.RetryRunner{
		Inner:  inner,
		Retry:  domain.RetryConfig{MaxAttempts: 1, BaseDelaySec: 0},
		Logger: platform.NewLogger(io.Discard, false),
	}

	// when
	_, err := runner.Run(context.Background(), "test", io.Discard,
		port.WithAllowedTools("mcp__linear__list_issues"))

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(inner.lastOpts.AllowedTools) != 1 || inner.lastOpts.AllowedTools[0] != "mcp__linear__list_issues" {
		t.Errorf("expected forwarded allowed tools, got %v", inner.lastOpts.AllowedTools)
	}
}

func TestRetryRunner_TimeoutStopsHangingInner(t *testing.T) {
	// given: inner blocks forever until context is cancelled
	inner := &hangingRunner{}
	runner := &session.RetryRunner{
		Inner:   inner,
		Retry:   domain.RetryConfig{MaxAttempts: 100, BaseDelaySec: 0},
		Timeout: 100 * time.Millisecond,
		Logger:  platform.NewLogger(io.Discard, false),
	}

	// when
	start := time.Now()
	_, err := runner.Run(context.Background(), "test", io.Discard)
	elapsed := time.Since(start)

	// then: must terminate within a reasonable bound (not 100 retries)
	if err == nil {
		t.Fatal("expected error from timeout")
	}
	if elapsed > 2*time.Second {
		t.Errorf("timeout did not stop loop: elapsed %v", elapsed)
	}
	if inner.calls > 5 {
		t.Errorf("expected few calls before timeout, got %d", inner.calls)
	}
}

// hangingRunner blocks until context is cancelled, simulating a hanging subprocess.
type hangingRunner struct {
	calls int
}

func (h *hangingRunner) Run(ctx context.Context, _ string, _ io.Writer, _ ...port.RunOption) (string, error) {
	h.calls++
	<-ctx.Done()
	return "", ctx.Err()
}

func TestRetryRunner_MaxAttemptsLessThanOne_DefaultsToOne(t *testing.T) {
	// given
	inner := &fakeRunner{failN: 100}
	runner := &session.RetryRunner{
		Inner:  inner,
		Retry:  domain.RetryConfig{MaxAttempts: 0, BaseDelaySec: 0},
		Logger: platform.NewLogger(io.Discard, false),
	}

	// when
	_, err := runner.Run(context.Background(), "test", io.Discard)

	// then
	if err == nil {
		t.Fatal("expected error")
	}
	if inner.calls != 1 {
		t.Errorf("expected 1 call (defaulted from 0), got %d", inner.calls)
	}
}
