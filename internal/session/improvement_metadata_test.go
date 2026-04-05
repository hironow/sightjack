package session

import (
	"testing"

	"github.com/hironow/sightjack/internal/domain"
)

// white-box-reason: session internals: tests unexported corrective metadata matching helper

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
		Severity:         domain.SeverityMedium,
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
		Severity:         domain.SeverityHigh,
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

func TestCorrectionMetadataForWave_AcceptsLegacyV1WithoutSchemaVersion(t *testing.T) {
	wave := domain.Wave{
		ID:          "auth-w1",
		ClusterName: "Auth",
		Actions: []domain.WaveAction{
			{IssueID: "ENG-3"},
		},
	}
	feedback := []*DMail{{
		Name:   "feedback-legacy",
		Issues: []string{"ENG-3"},
		Metadata: map[string]string{
			domain.MetadataFailureType:      string(domain.FailureTypeScopeViolation),
			domain.MetadataSeverity:         "HIGH",
			domain.MetadataCorrelationID:    "corr-legacy",
			domain.MetadataCorrectiveAction: "retry",
		},
	}}

	got := correctionMetadataForWave(feedback, wave)

	if got.ConsumerSchemaVersion() != domain.ImprovementSchemaVersion {
		t.Fatalf("ConsumerSchemaVersion = %q, want %q", got.ConsumerSchemaVersion(), domain.ImprovementSchemaVersion)
	}
	if got.Severity != domain.SeverityHigh {
		t.Fatalf("Severity = %q, want %q", got.Severity, domain.SeverityHigh)
	}
	if got.Outcome != domain.ImprovementOutcomePending {
		t.Fatalf("Outcome = %q, want %q", got.Outcome, domain.ImprovementOutcomePending)
	}
}
