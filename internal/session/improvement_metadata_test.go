package session

import (
	"testing"

	"github.com/hironow/sightjack/internal/domain"
)

func TestCorrectionMetadataForWave_PrefersWaveReference(t *testing.T) {
	wave := domain.Wave{
		ID:          "auth-w1",
		ClusterName: "Auth",
		Actions: []domain.WaveAction{
			{IssueID: "ENG-1"},
		},
	}
	meta := domain.CorrectionMetadata{
		FailureType:      domain.FailureTypeScopeViolation,
		TargetAgent:      "sightjack",
		CorrelationID:    "corr-wave",
		CorrectiveAction: "retry",
		RetryAllowed:     domain.BoolPtr(true),
	}
	feedback := []*DMail{{
		Name:     "feedback-1",
		Wave:     &domain.WaveReference{ID: domain.WaveKey(wave)},
		Metadata: meta.Apply(nil),
	}}

	got := correctionMetadataForWave(feedback, wave)

	if got.CorrelationID != "corr-wave" {
		t.Fatalf("CorrelationID = %q, want corr-wave", got.CorrelationID)
	}
	if got.TargetAgent != "" {
		t.Fatalf("TargetAgent = %q, want empty", got.TargetAgent)
	}
	if got.RetryAllowed == nil || !*got.RetryAllowed {
		t.Fatal("RetryAllowed = nil/false, want true")
	}
}

func TestCorrectionMetadataForWave_FallsBackToIssueMatch(t *testing.T) {
	wave := domain.Wave{
		ID:          "auth-w1",
		ClusterName: "Auth",
		Actions: []domain.WaveAction{
			{IssueID: "ENG-2"},
		},
	}
	meta := domain.CorrectionMetadata{
		FailureType:      domain.FailureTypeMissingAcceptance,
		TargetAgent:      "sightjack",
		CorrelationID:    "corr-issue",
		CorrectiveAction: "retry",
		RetryAllowed:     domain.BoolPtr(false),
		EscalationReason: "recurrence-threshold",
	}
	feedback := []*DMail{{
		Name:     "feedback-2",
		Issues:   []string{"ENG-2"},
		Metadata: meta.Apply(nil),
	}}

	got := correctionMetadataForWave(feedback, wave)

	if got.CorrelationID != "corr-issue" {
		t.Fatalf("CorrelationID = %q, want corr-issue", got.CorrelationID)
	}
	if got.TargetAgent != "" {
		t.Fatalf("TargetAgent = %q, want empty", got.TargetAgent)
	}
	if got.RetryAllowed == nil || *got.RetryAllowed {
		t.Fatal("RetryAllowed = nil/true, want false")
	}
	if got.EscalationReason != "recurrence-threshold" {
		t.Fatalf("EscalationReason = %q, want recurrence-threshold", got.EscalationReason)
	}
}
