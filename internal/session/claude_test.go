package session_test

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/platform"
	"github.com/hironow/sightjack/internal/session"
)

// TestClaudeAdapter_RunDetailedReturnsErrMCPPivotDeprecated is the
// canonical post jun15 MCP pivot assertion (refs/issues/0027): the
// previous suite of args / retry / stream tests exercised an exec
// path that has been removed. This single test pins the behavior
// callers can rely on — every invocation short-circuits with
// session.ErrMCPPivotDeprecated so operators are routed to the
// human-initiated claude code /sightjack-scan skill instead.
func TestClaudeAdapter_RunDetailedReturnsErrMCPPivotDeprecated(t *testing.T) {
	// given
	cfg := &domain.Config{
		ClaudeCmd:  "claude",
		Model:      "opus",
		TimeoutSec: 10,
		Retry:      domain.RetryConfig{MaxAttempts: 1, BaseDelaySec: 0},
	}
	logger := platform.NewLogger(io.Discard, false)
	adapter := session.NewClaudeAdapter(cfg, logger)

	// when
	_, err := adapter.Run(context.Background(), "anything", io.Discard)

	// then
	if !errors.Is(err, session.ErrMCPPivotDeprecated) {
		t.Errorf("Run() error = %v, want ErrMCPPivotDeprecated", err)
	}
}

func TestRunClaudeDryRun(t *testing.T) {
	dir := t.TempDir()
	cfg := &domain.Config{ClaudeCmd: "claude"}
	prompt := "test prompt content"
	outDir := dir + "/dryrun"

	err := session.RunClaudeDryRun(cfg, prompt, outDir, "classify", platform.NewLogger(io.Discard, false))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(outDir + "/classify_prompt.md")
	if err != nil {
		t.Fatalf("failed to read prompt file: %v", err)
	}
	if string(data) != prompt {
		t.Errorf("expected %q, got %q", prompt, string(data))
	}
}

func TestRunClaudeDryRun_UniqueNames(t *testing.T) {
	// given: two dry-run calls with different names to the same dir
	dir := t.TempDir()
	cfg := &domain.Config{ClaudeCmd: "claude"}

	// when
	if err := session.RunClaudeDryRun(cfg, "prompt A", dir, "wave_00_auth", platform.NewLogger(io.Discard, false)); err != nil {
		t.Fatal(err)
	}
	if err := session.RunClaudeDryRun(cfg, "prompt B", dir, "wave_01_api", platform.NewLogger(io.Discard, false)); err != nil {
		t.Fatal(err)
	}

	// then: both files exist with correct content
	dataA, err := os.ReadFile(dir + "/wave_00_auth_prompt.md")
	if err != nil {
		t.Fatalf("wave_00_auth prompt missing: %v", err)
	}
	if string(dataA) != "prompt A" {
		t.Errorf("expected 'prompt A', got %q", string(dataA))
	}

	dataB, err := os.ReadFile(dir + "/wave_01_api_prompt.md")
	if err != nil {
		t.Fatalf("wave_01_api prompt missing: %v", err)
	}
	if string(dataB) != "prompt B" {
		t.Errorf("expected 'prompt B', got %q", string(dataB))
	}
}

func TestRetryRunner_NoRetryOnCancel_WithFakeCmd(t *testing.T) {
	callCount := 0
	cleanup := session.SetNewCmd(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		callCount++
		return exec.CommandContext(ctx, "false")
	})
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	cfg := &domain.Config{
		ClaudeCmd:  "claude",
		TimeoutSec: 10,
		Retry:      domain.RetryConfig{MaxAttempts: 3, BaseDelaySec: 0},
	}
	logger := platform.NewLogger(io.Discard, false)
	adapter := session.NewClaudeAdapter(cfg, logger)
	retrier := session.NewRetryRunner(adapter, cfg, logger)

	_, err := retrier.Run(ctx, "test", io.Discard)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
	if callCount > 1 {
		t.Errorf("expected no retry on cancellation, got %d calls", callCount)
	}
}

func TestClaudeAdapter_GracefulShutdownOnCancel(t *testing.T) {
	// given: a command that runs for a long time (sleep 30)
	cleanup := session.SetNewCmd(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sleep", "30")
	})
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	cfg := &domain.Config{
		ClaudeCmd:  "sleep",
		TimeoutSec: 30,
		Retry:      domain.RetryConfig{MaxAttempts: 1},
	}
	logger := platform.NewLogger(io.Discard, false)
	adapter := session.NewClaudeAdapter(cfg, logger)

	// when: cancel context after 100ms
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := adapter.Run(ctx, "test", io.Discard)
	elapsed := time.Since(start)

	// then: should terminate with error (context cancelled)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}

	// then: should terminate within 10 seconds (not hang forever)
	if elapsed > 10*time.Second {
		t.Errorf("command took too long to terminate: %v (expected < 10s)", elapsed)
	}
}

func TestClaudeAdapter_NoStrictMCPConfig_WhenFileAbsent(t *testing.T) {
	// given: no .mcp.json
	workDir := t.TempDir()

	var capturedArgs []string
	cleanup := session.SetNewCmd(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		capturedArgs = args
		return exec.CommandContext(ctx, "echo", "ok")
	})
	defer cleanup()

	cfg := &domain.Config{
		ClaudeCmd:  "claude",
		Model:      "opus",
		TimeoutSec: 10,
		Retry:      domain.RetryConfig{MaxAttempts: 1, BaseDelaySec: 0},
	}
	logger := platform.NewLogger(io.Discard, false)
	adapter := session.NewClaudeAdapter(cfg, logger)

	// when: run with empty WorkDir (no .mcp.json)
	adapter.Run(context.Background(), "test", io.Discard,
		session.WithWorkDir(workDir))

	// then: --strict-mcp-config should NOT be in args
	argsStr := strings.Join(capturedArgs, " ")
	if strings.Contains(argsStr, "--strict-mcp-config") {
		t.Errorf("--strict-mcp-config should not be present without .mcp.json: %v", capturedArgs)
	}
}
