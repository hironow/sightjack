package policy_test

import (
	"testing"

	"github.com/hironow/sightjack/internal/harness/policy"
)

func TestIsConvergenceKind(t *testing.T) {
	if !policy.IsConvergenceKind("convergence") {
		t.Fatal("IsConvergenceKind(convergence) = false, want true")
	}
	if policy.IsConvergenceKind("design-feedback") {
		t.Fatal("IsConvergenceKind(design-feedback) = true, want false")
	}
}

func TestBuildConvergenceSummary(t *testing.T) {
	got := policy.BuildConvergenceSummary([]string{"conv-1", "conv-2"})
	want := "[CONVERGENCE] Convergence signal received: conv-1, conv-2"
	if got != want {
		t.Fatalf("BuildConvergenceSummary() = %q, want %q", got, want)
	}
}
