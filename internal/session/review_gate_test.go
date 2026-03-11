package session_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
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
	gate := domain.GateConfig{} // ReviewCmd empty
	assistant := &domain.Config{}

	// when — runner is nil because it's never used when ReviewCmd is empty
	passed, err := session.RunReviewGate(ctx, gate, assistant, nil, t.TempDir(), nil)

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
	gate := domain.GateConfig{ReviewCmd: "echo 'all good'"}
	assistant := &domain.Config{ClaudeCmd: "true", TimeoutSec: 30}

	// when — runner is nil because fix is never reached when review passes
	passed, err := session.RunReviewGate(ctx, gate, assistant, nil, t.TempDir(), nil)

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
	gate := domain.GateConfig{ReviewCmd: "echo 'found issues' && exit 1"}
	assistant := &domain.Config{ClaudeCmd: "true", TimeoutSec: 30}
	dir := t.TempDir()
	initGitRepo(t, dir)
	runner := session.NewClaudeAdapter(assistant, nil)

	// when
	passed, err := session.RunReviewGate(ctx, gate, assistant, runner, dir, nil)

	// then — should return false (not passed), no error
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if passed {
		t.Error("expected passed=false after exhausting review cycles")
	}
}

func TestRunReviewGate_BudgetExceeded(t *testing.T) {
	// given — budget=1, review always fails
	ctx := context.Background()
	gate := domain.GateConfig{ReviewCmd: "echo 'error' && exit 1", ReviewBudget: 1}
	assistant := &domain.Config{ClaudeCmd: "true", TimeoutSec: 30}
	dir := t.TempDir()
	initGitRepo(t, dir)

	// when — budget=1 means no fix cycle, runner is never used
	passed, err := session.RunReviewGate(ctx, gate, assistant, nil, dir, nil)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if passed {
		t.Error("expected passed=false with budget=1 and failing review")
	}
}

func TestRunReviewGate_BudgetZeroUsesDefault(t *testing.T) {
	// given — budget=0 means use default (3)
	ctx := context.Background()
	gate := domain.GateConfig{ReviewCmd: "echo ok"}
	assistant := &domain.Config{TimeoutSec: 30}

	// when — review passes immediately, runner is never used
	passed, err := session.RunReviewGate(ctx, gate, assistant, nil, t.TempDir(), nil)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !passed {
		t.Error("expected passed=true for passing review")
	}
}

func TestRunReviewGate_ReviewCommentsPropagatedToFix(t *testing.T) {
	// given — review outputs specific comments, verify they reach the fix prompt
	dir := t.TempDir()
	initGitRepo(t, dir)

	promptCapture := filepath.Join(dir, "captured-prompt.txt")

	reviewScript := filepath.Join(dir, "review.sh")
	os.WriteFile(reviewScript, []byte("#!/bin/bash\necho 'UNIQUE-SIGHTJACK-REVIEW-COMMENT-ABC-123'\nexit 1\n"), 0755)

	// Fake claude captures -p argument to file
	fakeClaudeScript := filepath.Join(dir, "fake-claude.sh")
	os.WriteFile(fakeClaudeScript, []byte(`#!/bin/bash
while [ $# -gt 0 ]; do
  if [ "$1" = "-p" ]; then
    echo "$2" > `+promptCapture+`
    break
  fi
  shift
done
exit 0
`), 0755)

	gate := domain.GateConfig{ReviewCmd: reviewScript, ReviewBudget: 2}
	assistant := &domain.Config{ClaudeCmd: fakeClaudeScript, Model: "opus", TimeoutSec: 30}
	runner := session.NewClaudeAdapter(assistant, nil)
	ctx := context.Background()

	// when — review fails, fix is called with review comments in prompt
	session.RunReviewGate(ctx, gate, assistant, runner, dir, nil)

	// then — captured prompt should contain the review comments
	captured, err := os.ReadFile(promptCapture)
	if err != nil {
		t.Fatalf("fix was not called (no captured prompt): %v", err)
	}
	if !strings.Contains(string(captured), "UNIQUE-SIGHTJACK-REVIEW-COMMENT-ABC-123") {
		t.Errorf("review comments not propagated to fix prompt, got: %s", string(captured))
	}
}

func TestRunReviewGate_FixCycleExecuted(t *testing.T) {
	// given — review fails once, then passes after fix
	dir := t.TempDir()
	initGitRepo(t, dir)

	callCount := filepath.Join(dir, "call-count")
	os.WriteFile(callCount, []byte("0"), 0644)

	// Review script: fail first call, pass second
	reviewScript := filepath.Join(dir, "review.sh")
	os.WriteFile(reviewScript, []byte(`#!/bin/bash
COUNT=$(cat `+callCount+`)
COUNT=$((COUNT + 1))
echo $COUNT > `+callCount+`
if [ $COUNT -eq 1 ]; then
  echo "fix this naming issue"
  exit 1
fi
exit 0
`), 0755)

	// Fake claude: just succeed (noop fix)
	fakeClaudeScript := filepath.Join(dir, "fake-claude.sh")
	os.WriteFile(fakeClaudeScript, []byte("#!/bin/bash\nexit 0\n"), 0755)

	gate := domain.GateConfig{ReviewCmd: reviewScript, ReviewBudget: 3}
	assistant := &domain.Config{ClaudeCmd: fakeClaudeScript, Model: "opus", TimeoutSec: 30}
	runner := session.NewClaudeAdapter(assistant, nil)
	ctx := context.Background()

	// when — review fail → fix (fake claude) → review pass
	passed, err := session.RunReviewGate(ctx, gate, assistant, runner, dir, nil)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !passed {
		t.Error("expected passed=true after fix cycle resolves review")
	}
}

func TestRunReviewGate_FixFailure_ReturnsFalse(t *testing.T) {
	// given — review fails, fix also fails (claude exits non-zero)
	dir := t.TempDir()
	initGitRepo(t, dir)

	reviewScript := filepath.Join(dir, "review.sh")
	os.WriteFile(reviewScript, []byte("#!/bin/bash\necho 'issue'\nexit 1\n"), 0755)

	fakeClaudeScript := filepath.Join(dir, "fake-claude.sh")
	os.WriteFile(fakeClaudeScript, []byte("#!/bin/bash\nexit 1\n"), 0755)

	gate := domain.GateConfig{ReviewCmd: reviewScript, ReviewBudget: 2}
	assistant := &domain.Config{ClaudeCmd: fakeClaudeScript, Model: "opus", TimeoutSec: 30}
	runner := session.NewClaudeAdapter(assistant, nil)
	ctx := context.Background()

	// when — fix fails
	passed, err := session.RunReviewGate(ctx, gate, assistant, runner, dir, nil)

	// then — should return false (not error)
	if err != nil {
		t.Fatalf("fix failure should not be infrastructure error: %v", err)
	}
	if passed {
		t.Error("expected passed=false when fix fails")
	}
}
