package sightjack

import (
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestBuildClaudeArgs(t *testing.T) {
	cfg := &Config{
		Claude: ClaudeConfig{
			Command: "claude",
			Model:   "opus",
		},
	}
	prompt := "Analyze these issues"

	args := BuildClaudeArgs(cfg, prompt)

	expected := []string{"--print", "--model", "opus", "-p", "Analyze these issues"}
	if len(args) != len(expected) {
		t.Fatalf("expected %d args, got %d: %v", len(expected), len(args), args)
	}
	for i, e := range expected {
		if args[i] != e {
			t.Errorf("arg[%d]: expected %q, got %q", i, e, args[i])
		}
	}
}

func TestBuildClaudeArgs_NoModel(t *testing.T) {
	cfg := &Config{
		Claude: ClaudeConfig{
			Command: "claude",
			Model:   "",
		},
	}
	prompt := "test prompt"

	args := BuildClaudeArgs(cfg, prompt)

	for _, a := range args {
		if a == "--model" {
			t.Error("--model should not be present when model is empty")
		}
	}
}

func TestRunClaudeDryRun(t *testing.T) {
	dir := t.TempDir()
	cfg := &Config{Claude: ClaudeConfig{Command: "claude"}}
	prompt := "test prompt content"
	outDir := dir + "/dryrun"

	err := RunClaudeDryRun(cfg, prompt, outDir, "classify")

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
	cfg := &Config{Claude: ClaudeConfig{Command: "claude"}}

	// when
	if err := RunClaudeDryRun(cfg, "prompt A", dir, "wave_00_auth"); err != nil {
		t.Fatal(err)
	}
	if err := RunClaudeDryRun(cfg, "prompt B", dir, "wave_01_api"); err != nil {
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

func TestRunClaudeRetriesOnFailure(t *testing.T) {
	callCount := 0
	newCmd = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		callCount++
		if callCount < 3 {
			return exec.Command("false") // exits non-zero
		}
		return exec.Command("echo", "success")
	}
	defer func() { newCmd = defaultNewCmd }()

	cfg := &Config{
		Claude: ClaudeConfig{Command: "claude", TimeoutSec: 10},
		Retry:  RetryConfig{MaxAttempts: 3, BaseDelaySec: 0}, // 0 delay for fast test
	}
	output, err := RunClaude(context.Background(), cfg, "test", io.Discard)
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
	newCmd = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		callCount++
		return exec.Command("false")
	}
	defer func() { newCmd = defaultNewCmd }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	cfg := &Config{
		Claude: ClaudeConfig{Command: "claude", TimeoutSec: 10},
		Retry:  RetryConfig{MaxAttempts: 3, BaseDelaySec: 0},
	}
	_, err := RunClaude(ctx, cfg, "test", io.Discard)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
	if callCount > 1 {
		t.Errorf("expected no retry on cancellation, got %d calls", callCount)
	}
}

func TestRunClaudeExhaustsRetries(t *testing.T) {
	callCount := 0
	newCmd = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		callCount++
		return exec.Command("false")
	}
	defer func() { newCmd = defaultNewCmd }()

	cfg := &Config{
		Claude: ClaudeConfig{Command: "claude", TimeoutSec: 10},
		Retry:  RetryConfig{MaxAttempts: 2, BaseDelaySec: 0},
	}
	_, err := RunClaude(context.Background(), cfg, "test", io.Discard)
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls, got %d", callCount)
	}
}
