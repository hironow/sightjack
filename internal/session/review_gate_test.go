package session_test

import (
	"context"
	"os/exec"
	"testing"

	"github.com/hironow/sightjack"
	"github.com/hironow/sightjack/internal/session"
)

// initGitRepo creates a minimal git repo in dir with an initial commit.
func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "test@test.com"},
		{"config", "user.name", "Test"},
		{"commit", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func TestRunReviewGate_SkippedWhenNoCmd(t *testing.T) {
	// given
	ctx := context.Background()
	cfg := &sightjack.Config{} // ReviewCmd empty

	// when
	passed, err := session.RunReviewGate(ctx, cfg, t.TempDir(), nil)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !passed {
		t.Error("expected passed=true when no review command configured")
	}
}

func TestRunReviewGate_PassingReview(t *testing.T) {
	// given
	ctx := context.Background()
	cfg := &sightjack.Config{
		Gate:   sightjack.GateConfig{ReviewCmd: "echo 'all good'"},
		Claude: sightjack.ClaudeConfig{Command: "true", TimeoutSec: 30},
	}

	// when
	passed, err := session.RunReviewGate(ctx, cfg, t.TempDir(), nil)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !passed {
		t.Error("expected passed=true for passing review")
	}
}

func TestRunReviewGate_FailingReviewExhaustedCycles(t *testing.T) {
	// given — review always fails (exit 1), claude fix is "true" (noop, does nothing)
	ctx := context.Background()
	cfg := &sightjack.Config{
		Gate:   sightjack.GateConfig{ReviewCmd: "echo 'found issues' && exit 1"},
		Claude: sightjack.ClaudeConfig{Command: "true", TimeoutSec: 30},
	}
	dir := t.TempDir()
	initGitRepo(t, dir)

	// when
	passed, err := session.RunReviewGate(ctx, cfg, dir, nil)

	// then — should return false (not passed), no error
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if passed {
		t.Error("expected passed=false after exhausting review cycles")
	}
}
