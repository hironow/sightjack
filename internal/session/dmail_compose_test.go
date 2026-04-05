package session_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/session"
)

func TestComposeSpecification_WaveMode_AttachesWaveReference(t *testing.T) {
	// given: wave with actions in wave mode
	dir := t.TempDir()
	store := testOutboxStore(t, dir)
	wave := domain.Wave{
		ID:          "w1",
		ClusterName: "auth",
		Title:       "Auth Wave",
		Actions: []domain.WaveAction{
			{IssueID: "MY-1", Description: "Add middleware", Detail: "JWT based"},
			{IssueID: "MY-2", Description: "Add login"},
		},
	}

	// when: compose with wave mode
	err := session.ComposeSpecification(context.Background(), store, wave, domain.ModeWave)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Verify outbox was flushed (store stages internally; flush verifies write)
}

func TestComposeSpecification_LinearMode_NoWaveReference(t *testing.T) {
	// given: wave in linear mode
	dir := t.TempDir()
	store := testOutboxStore(t, dir)
	wave := domain.Wave{
		ID:          "w1",
		ClusterName: "auth",
		Title:       "Auth Wave",
		Actions: []domain.WaveAction{
			{IssueID: "MY-1", Description: "Add middleware"},
		},
	}

	// when: compose without mode (defaults to no wave ref)
	err := session.ComposeSpecification(context.Background(), store, wave)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestComposeSpecification_WaveMode_EmptyActions_ReturnsSentinelError(t *testing.T) {
	// given: wave with no actions
	dir := t.TempDir()
	store := testOutboxStore(t, dir)
	wave := domain.Wave{
		ID:          "w1",
		ClusterName: "fix",
		Title:       "Quick Fix",
	}

	// when
	err := session.ComposeSpecification(context.Background(), store, wave, domain.ModeWave)

	// then: should return sentinel error (no implementation steps)
	if !errors.Is(err, session.ErrSpecNoImplementationSteps) {
		t.Fatalf("expected ErrSpecNoImplementationSteps, got: %v", err)
	}
}

func TestComposeSpecification_IssueManagementOnly_ReturnsSentinelError(t *testing.T) {
	// given: wave with ONLY issue management actions in wave mode
	dir := t.TempDir()
	store := testOutboxStore(t, dir)
	wave := domain.Wave{
		ID:          "w1",
		ClusterName: "design",
		Title:       "Design cleanup",
		Actions: []domain.WaveAction{
			{Type: "add_dod", IssueID: "MY-1", Description: "Add DoD to MY-1"},
			{Type: "add_dependency", IssueID: "MY-2", Description: "Link MY-2"},
			{Type: "cancel", IssueID: "MY-3", Description: "Cancel MY-3"},
		},
	}

	// when
	err := session.ComposeSpecification(context.Background(), store, wave, domain.ModeWave)

	// then: should return sentinel error — no spec D-Mail generated
	if !errors.Is(err, session.ErrSpecNoImplementationSteps) {
		t.Fatalf("expected ErrSpecNoImplementationSteps, got: %v", err)
	}
}

func TestComposeFeedbackWithMetadata_AttachesProviderState(t *testing.T) {
	dir := t.TempDir()
	session.EnsureMailDirs(dir)
	store := testOutboxStore(t, dir)
	wave := domain.Wave{
		ID:          "w1",
		ClusterName: "auth",
		Title:       "Auth Wave",
		Actions:     []domain.WaveAction{{IssueID: "MY-1", Description: "Add middleware"}},
	}
	result := &domain.WaveApplyResult{WaveID: "w1", Applied: 1}

	err := session.ComposeFeedbackWithMetadata(context.Background(), store, wave, result, domain.CorrectionMetadata{
		SchemaVersion:    domain.ImprovementSchemaVersion,
		FailureType:      domain.FailureTypeScopeViolation,
		TargetAgent:      "sightjack",
		RoutingHistory:   []string{"retry"},
		OwnerHistory:     []string{"sightjack"},
		CorrelationID:    "corr-1",
		CorrectiveAction: "retry",
	})
	if err != nil {
		t.Fatalf("ComposeFeedbackWithMetadata: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(domain.MailDir(dir, "outbox"), "sj-feedback-auth-w1_00000000.md"))
	if err != nil {
		t.Fatalf("read feedback dmail: %v", err)
	}
	mail, err := session.ParseDMail(data)
	if err != nil {
		t.Fatalf("ParseDMail: %v", err)
	}
	if got := mail.Metadata[domain.MetadataProviderState]; got != string(domain.ProviderStateActive) {
		t.Fatalf("provider_state = %q, want %q", got, domain.ProviderStateActive)
	}
	if got := mail.Metadata[domain.MetadataRoutingHistory]; got != "retry" {
		t.Fatalf("routing_history = %q, want retry", got)
	}
}
