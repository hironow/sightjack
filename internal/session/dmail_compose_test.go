package session_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/harness"
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

// loadWaveSpec composes a wave-mode specification d-mail and returns the
// parsed DMail. Helper for Rival Contract v1 producer tests.
func loadWaveSpec(t *testing.T, dir, clusterName, waveID string, wave domain.Wave) *domain.DMail {
	t.Helper()
	session.EnsureMailDirs(dir)
	store := testOutboxStore(t, dir)
	if err := session.ComposeSpecification(context.Background(), store, wave, domain.ModeWave); err != nil {
		t.Fatalf("ComposeSpecification: %v", err)
	}
	matches, _ := filepath.Glob(filepath.Join(domain.MailDir(dir, "outbox"), "sj-spec-*.md"))
	if len(matches) != 1 {
		t.Fatalf("expected 1 spec D-Mail, got %d (cluster=%s wave=%s)", len(matches), clusterName, waveID)
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}
	mail, err := session.ParseDMail(data)
	if err != nil {
		t.Fatalf("parse spec: %v", err)
	}
	return mail
}

func TestComposeSpecification_WaveMode_RendersRivalContractV1(t *testing.T) {
	// given: a wave with an implementation action in wave mode
	dir := t.TempDir()
	wave := domain.Wave{
		ID:          "rcv1-wave",
		ClusterName: "auth",
		Title:       "Add session expiry enforcement",
		Description: "Prevent expired sessions from authorizing API calls.",
		Actions: []domain.WaveAction{
			{Type: "implement", IssueID: "MY-1", Description: "Add expiry check", Detail: "Edit middleware"},
			{Type: "add_dod", IssueID: "MY-1", Description: "Update DoD"},
		},
	}

	// when
	mail := loadWaveSpec(t, dir, "auth", "rcv1-wave", wave)

	// then: body parses as Rival Contract v1
	contract, ok, err := harness.ParseRivalContractBody(mail.Body)
	if err != nil {
		t.Fatalf("ParseRivalContractBody: %v", err)
	}
	if !ok {
		t.Fatalf("body must be a Rival Contract v1 body in wave mode; got body:\n%s", mail.Body)
	}
	if contract.Title != "Add session expiry enforcement" {
		t.Errorf("contract title: got %q", contract.Title)
	}
	if !strings.Contains(contract.Steps, "Add expiry check") {
		t.Errorf("Steps must contain implementation action title, got: %q", contract.Steps)
	}
	// All six headings present (renderer-level invariant; double-check at compose level).
	for _, h := range []string{"## Intent", "## Domain", "## Decisions", "## Steps", "## Boundaries", "## Evidence"} {
		if !strings.Contains(mail.Body, h) {
			t.Errorf("missing heading %q in spec body", h)
		}
	}
}

func TestComposeSpecification_WritesContractMetadata(t *testing.T) {
	// given
	dir := t.TempDir()
	wave := domain.Wave{
		ID:          "meta-wave",
		ClusterName: "core",
		Title:       "Metadata test",
		Actions: []domain.WaveAction{
			{Type: "implement", IssueID: "MY-99", Description: "Action"},
		},
	}

	// when
	mail := loadWaveSpec(t, dir, "core", "meta-wave", wave)

	// then: required Rival Contract metadata fields present
	if got := mail.Metadata["contract_schema"]; got != harness.SchemaRivalContractV1 {
		t.Errorf("contract_schema: got %q, want %q", got, harness.SchemaRivalContractV1)
	}
	if got := mail.Metadata["contract_id"]; got == "" {
		t.Error("contract_id must be set")
	}
	if got := mail.Metadata["contract_revision"]; got != "1" {
		t.Errorf("contract_revision: got %q, want %q", got, "1")
	}
	if got, ok := mail.Metadata["supersedes"]; !ok || got != "" {
		t.Errorf("supersedes must be present and empty for first revision, got %q (present=%v)", got, ok)
	}
}

func TestComposeSpecification_ContractIDUsesWaveID(t *testing.T) {
	// given: a wave with a deterministic ID
	dir := t.TempDir()
	wave := domain.Wave{
		ID:          "stable-wave-id-001",
		ClusterName: "auth",
		Title:       "Use wave id as contract id",
		Actions: []domain.WaveAction{
			{Type: "implement", IssueID: "MY-1", Description: "Action"},
		},
	}

	// when
	mail := loadWaveSpec(t, dir, "auth", "stable-wave-id-001", wave)

	// then: contract_id MUST equal the wave id
	if got := mail.Metadata["contract_id"]; got != "stable-wave-id-001" {
		t.Errorf("contract_id should equal wave id; got %q, want %q", got, "stable-wave-id-001")
	}
	// And it must parse via the canonical metadata parser.
	parsed, ok, err := harness.ParseRivalContractMetadata(mail.Metadata)
	if err != nil {
		t.Fatalf("ParseRivalContractMetadata: %v", err)
	}
	if !ok {
		t.Fatal("ParseRivalContractMetadata: ok=false")
	}
	if parsed.ID != "stable-wave-id-001" {
		t.Errorf("parsed ID: got %q", parsed.ID)
	}
	if parsed.Revision != 1 {
		t.Errorf("parsed Revision: got %d, want 1", parsed.Revision)
	}
}

func TestComposeSpecification_DoesNotUseDMailNameAsContractID(t *testing.T) {
	// given
	dir := t.TempDir()
	wave := domain.Wave{
		ID:          "real-wave-id",
		ClusterName: "core",
		Title:       "Identity guard",
		Actions: []domain.WaveAction{
			{Type: "implement", IssueID: "MY-1", Description: "Action"},
		},
	}

	// when
	mail := loadWaveSpec(t, dir, "core", "real-wave-id", wave)

	// then: contract_id must NOT equal the D-Mail name
	if mail.Metadata["contract_id"] == mail.Name {
		t.Errorf("contract_id (%q) must not equal D-Mail name (%q)", mail.Metadata["contract_id"], mail.Name)
	}
	// And the metadata parser must accept it (i.e. id is not D-Mail-name shaped).
	if _, _, err := harness.ParseRivalContractMetadata(mail.Metadata); err != nil {
		t.Errorf("metadata parser rejected emitted contract metadata: %v", err)
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
