package session_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/session"
)

func TestWriteCorrectionInsight_AppendsImprovementInsight(t *testing.T) {
	base := t.TempDir()
	if err := os.MkdirAll(filepath.Join(base, ".siren", "insights"), 0o755); err != nil {
		t.Fatalf("mkdir insights: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(base, ".siren", ".run"), 0o755); err != nil {
		t.Fatalf("mkdir run: %v", err)
	}
	w := session.NewInsightWriter(filepath.Join(base, ".siren", "insights"), filepath.Join(base, ".siren", ".run"))
	mail := &session.DMail{
		Name: "feedback-1",
		Metadata: map[string]string{
			domain.MetadataFailureType:         string(domain.FailureTypeScopeViolation),
			domain.MetadataSeverity:            "medium",
			domain.MetadataTargetAgent:         "sightjack",
			domain.MetadataCorrectiveAction:    "retry",
			domain.MetadataProviderState:       string(domain.ProviderStateWaiting),
			domain.MetadataProviderReason:      domain.ProviderReasonRateLimit,
			domain.MetadataProviderRetryBudget: "0",
			domain.MetadataProviderResumeWhen:  domain.ResumeConditionProbeSucceeds,
			domain.MetadataOutcome:             string(domain.ImprovementOutcomePending),
		},
	}

	session.WriteCorrectionInsight(w, mail, &domain.NopLogger{})

	file, err := w.Read("improvement-loop.md")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(file.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(file.Entries))
	}
	if file.Entries[0].Title != "feedback-1" {
		t.Fatalf("title = %q, want feedback-1", file.Entries[0].Title)
	}
	if file.Entries[0].Extra["provider-state"] != string(domain.ProviderStateWaiting) {
		t.Fatalf("provider-state = %q, want %q", file.Entries[0].Extra["provider-state"], domain.ProviderStateWaiting)
	}
	if file.Entries[0].Extra["provider-resume-when"] != domain.ResumeConditionProbeSucceeds {
		t.Fatalf("provider-resume-when = %q, want %q", file.Entries[0].Extra["provider-resume-when"], domain.ResumeConditionProbeSucceeds)
	}
}
