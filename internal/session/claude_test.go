package session_test

import (
	"context"
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

func TestRunClaudeOnce_ArgsWithModel(t *testing.T) {
	// given: config with model set
	var capturedArgs []string
	cleanup := session.SetNewCmd(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		capturedArgs = args
		return exec.CommandContext(ctx, "echo", "ok")
	})
	defer cleanup()

	cfg := &domain.Config{
		Assistant: domain.AIAssistantConfig{Command: "claude", Model: "opus", TimeoutSec: 10},
		Retry:     domain.RetryConfig{MaxAttempts: 1, BaseDelaySec: 0},
	}

	// when
	session.RunClaudeOnce(context.Background(), cfg, "Analyze these issues", io.Discard, platform.NewLogger(io.Discard, false))

	// then
	expected := []string{"--model", "opus", "--output-format", "stream-json", "--dangerously-skip-permissions", "--print", "-p", "Analyze these issues"}
	if len(capturedArgs) != len(expected) {
		t.Fatalf("expected %d args, got %d: %v", len(expected), len(capturedArgs), capturedArgs)
	}
	for i, e := range expected {
		if capturedArgs[i] != e {
			t.Errorf("arg[%d]: expected %q, got %q", i, e, capturedArgs[i])
		}
	}
}

func TestRunClaudeOnce_ArgsWithoutModel(t *testing.T) {
	// given: config without model
	var capturedArgs []string
	cleanup := session.SetNewCmd(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		capturedArgs = args
		return exec.CommandContext(ctx, "echo", "ok")
	})
	defer cleanup()

	cfg := &domain.Config{
		Assistant: domain.AIAssistantConfig{Command: "claude", Model: "", TimeoutSec: 10},
		Retry:     domain.RetryConfig{MaxAttempts: 1, BaseDelaySec: 0},
	}

	// when
	session.RunClaudeOnce(context.Background(), cfg, "test prompt", io.Discard, platform.NewLogger(io.Discard, false))

	// then
	expected := []string{"--output-format", "stream-json", "--dangerously-skip-permissions", "--print", "-p", "test prompt"}
	if len(capturedArgs) != len(expected) {
		t.Fatalf("expected %d args, got %d: %v", len(expected), len(capturedArgs), capturedArgs)
	}
	for i, e := range expected {
		if capturedArgs[i] != e {
			t.Errorf("arg[%d]: expected %q, got %q", i, e, capturedArgs[i])
		}
	}
}

func TestRunClaudeDryRun(t *testing.T) {
	dir := t.TempDir()
	cfg := &domain.Config{Assistant: domain.AIAssistantConfig{Command: "claude"}}
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
	cfg := &domain.Config{Assistant: domain.AIAssistantConfig{Command: "claude"}}

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

func TestRunClaudeOnceNoRetry(t *testing.T) {
	callCount := 0
	cleanup := session.SetNewCmd(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		callCount++
		return exec.CommandContext(ctx, "false") // exits non-zero
	})
	defer cleanup()

	cfg := &domain.Config{
		Assistant: domain.AIAssistantConfig{Command: "claude", TimeoutSec: 10},
		Retry:     domain.RetryConfig{MaxAttempts: 3, BaseDelaySec: 0},
	}

	// when
	_, err := session.RunClaudeOnce(context.Background(), cfg, "test", io.Discard, platform.NewLogger(io.Discard, false))

	// then: should fail immediately without retrying
	if err == nil {
		t.Fatal("expected error from RunClaudeOnce")
	}
	if callCount != 1 {
		t.Errorf("RunClaudeOnce should not retry; expected 1 call, got %d", callCount)
	}
}

func TestRunClaudeRetriesOnFailure(t *testing.T) {
	callCount := 0
	cleanup := session.SetNewCmd(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		callCount++
		if callCount < 3 {
			return exec.CommandContext(ctx, "false") // exits non-zero
		}
		// Emit stream-json result with "success" so StreamReader can parse it.
		resultLine := `{"type":"result","subtype":"success","session_id":"fake","result":"success","is_error":false,"num_turns":1,"duration_ms":100,"total_cost_usd":0.0,"usage":{"input_tokens":1,"output_tokens":1},"stop_reason":"end_turn"}`
		return exec.CommandContext(ctx, "sh", "-c", "echo '"+resultLine+"'")
	})
	defer cleanup()

	cfg := &domain.Config{
		Assistant: domain.AIAssistantConfig{Command: "claude", TimeoutSec: 10},
		Retry:     domain.RetryConfig{MaxAttempts: 3, BaseDelaySec: 0}, // 0 delay for fast test
	}
	output, err := session.RunClaude(context.Background(), cfg, "test", io.Discard, platform.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if !strings.Contains(output, "success") {
		t.Errorf("expected 'success' in output, got %q", output)
	}
	if callCount != 3 {
		t.Errorf("expected 3 calls, got %d", callCount)
	}
}

func TestRunClaudeNoRetryOnCancel(t *testing.T) {
	callCount := 0
	cleanup := session.SetNewCmd(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		callCount++
		return exec.CommandContext(ctx, "false")
	})
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	cfg := &domain.Config{
		Assistant: domain.AIAssistantConfig{Command: "claude", TimeoutSec: 10},
		Retry:     domain.RetryConfig{MaxAttempts: 3, BaseDelaySec: 0},
	}
	_, err := session.RunClaude(ctx, cfg, "test", io.Discard, platform.NewLogger(io.Discard, false))
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
	if callCount > 1 {
		t.Errorf("expected no retry on cancellation, got %d calls", callCount)
	}
}

func TestRunClaudeOnce_ArgsWithAllowedTools(t *testing.T) {
	// given: config with model and allowed tools option
	var capturedArgs []string
	cleanup := session.SetNewCmd(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		capturedArgs = args
		return exec.CommandContext(ctx, "echo", "ok")
	})
	defer cleanup()

	cfg := &domain.Config{
		Assistant: domain.AIAssistantConfig{Command: "claude", Model: "opus", TimeoutSec: 10},
		Retry:     domain.RetryConfig{MaxAttempts: 1, BaseDelaySec: 0},
	}

	// when
	session.RunClaudeOnce(context.Background(), cfg, "test", io.Discard, platform.NewLogger(io.Discard, false),
		session.WithAllowedTools("mcp__linear__list_issues", "mcp__linear__get_issue", "Write"))

	// then: --allowedTools flag present with comma-separated tools
	found := false
	for i, arg := range capturedArgs {
		if arg == "--allowedTools" && i+1 < len(capturedArgs) {
			expected := "mcp__linear__list_issues,mcp__linear__get_issue,Write"
			if capturedArgs[i+1] != expected {
				t.Errorf("--allowedTools value: expected %q, got %q", expected, capturedArgs[i+1])
			}
			found = true
			break
		}
	}
	if !found {
		t.Errorf("--allowedTools flag not found in args: %v", capturedArgs)
	}
}

func TestRunClaude_ForwardsAllowedTools(t *testing.T) {
	// given: RunClaude with allowed tools option
	var capturedArgs []string
	cleanup := session.SetNewCmd(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		capturedArgs = args
		return exec.CommandContext(ctx, "echo", "ok")
	})
	defer cleanup()

	cfg := &domain.Config{
		Assistant: domain.AIAssistantConfig{Command: "claude", TimeoutSec: 10},
		Retry:     domain.RetryConfig{MaxAttempts: 1, BaseDelaySec: 0},
	}

	// when
	session.RunClaude(context.Background(), cfg, "test", io.Discard, platform.NewLogger(io.Discard, false),
		session.WithAllowedTools("mcp__linear__list_issues"))

	// then: --allowedTools forwarded to RunClaudeOnce
	found := false
	for i, arg := range capturedArgs {
		if arg == "--allowedTools" && i+1 < len(capturedArgs) {
			if capturedArgs[i+1] != "mcp__linear__list_issues" {
				t.Errorf("--allowedTools value: expected %q, got %q", "mcp__linear__list_issues", capturedArgs[i+1])
			}
			found = true
			break
		}
	}
	if !found {
		t.Errorf("--allowedTools flag not found in RunClaude args: %v", capturedArgs)
	}
}

func TestRunClaudeOnce_GracefulShutdownOnCancel(t *testing.T) {
	// given: a command that runs for a long time (sleep 30)
	cleanup := session.SetNewCmd(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sleep", "30")
	})
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	cfg := &domain.Config{
		Assistant: domain.AIAssistantConfig{Command: "sleep", TimeoutSec: 30},
		Retry:     domain.RetryConfig{MaxAttempts: 1},
	}

	// when: cancel context after 100ms
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := session.RunClaudeOnce(ctx, cfg, "test", io.Discard, platform.NewLogger(io.Discard, false))
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

func TestRunClaudeExhaustsRetries(t *testing.T) {
	callCount := 0
	cleanup := session.SetNewCmd(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		callCount++
		return exec.CommandContext(ctx, "false")
	})
	defer cleanup()

	cfg := &domain.Config{
		Assistant: domain.AIAssistantConfig{Command: "claude", TimeoutSec: 10},
		Retry:     domain.RetryConfig{MaxAttempts: 2, BaseDelaySec: 0},
	}
	_, err := session.RunClaude(context.Background(), cfg, "test", io.Discard, platform.NewLogger(io.Discard, false))
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls, got %d", callCount)
	}
}
