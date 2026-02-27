package session_test

import (
	"context"
	"testing"
	"time"

	"github.com/hironow/sightjack/internal/session"
)

func TestRunReview_PassingCommand(t *testing.T) {
	// given
	ctx := context.Background()
	cmd := "echo 'all good'" // exit 0

	// when
	result, err := session.RunReview(ctx, cmd, t.TempDir())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Passed {
		t.Error("expected Passed=true for exit code 0")
	}
	if result.Comments != "" {
		t.Errorf("expected empty Comments, got %q", result.Comments)
	}
}

func TestRunReview_FailingCommand(t *testing.T) {
	// given
	ctx := context.Background()
	cmd := "echo 'found issues' && exit 1" // exit 1

	// when
	result, err := session.RunReview(ctx, cmd, t.TempDir())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed {
		t.Error("expected Passed=false for exit code 1")
	}
	if result.Comments == "" {
		t.Error("expected non-empty Comments for failing review")
	}
}

func TestRunReview_EmptyCommand(t *testing.T) {
	// given
	ctx := context.Background()

	// when
	result, err := session.RunReview(ctx, "", t.TempDir())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Passed {
		t.Error("expected Passed=true for empty command")
	}
}

func TestRunReview_Timeout(t *testing.T) {
	// given
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	cmd := "sleep 10"

	// when
	_, err := session.RunReview(ctx, cmd, t.TempDir())

	// then
	if err == nil {
		t.Fatal("expected error for timed out command")
	}
}

func TestRunReview_RateLimitDetected(t *testing.T) {
	// given
	ctx := context.Background()
	cmd := "echo 'rate limit exceeded' && exit 1"

	// when
	_, err := session.RunReview(ctx, cmd, t.TempDir())

	// then
	if err == nil {
		t.Fatal("expected error for rate-limited command")
	}
}
