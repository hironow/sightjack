package session_test

import (
	"context"
	"strings"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/session"
)

func TestDMailStallEscalation_Constant(t *testing.T) {
	// given/when: access the constant
	kind := session.DMailStallEscalation

	// then
	if kind == "" {
		t.Error("DMailStallEscalation: constant must not be empty")
	}
	if string(kind) != "stall-escalation" {
		t.Errorf("DMailStallEscalation = %q, want %q", kind, "stall-escalation")
	}
}

func TestStallEscalationBody_ContainsWaveAndReason(t *testing.T) {
	// given
	wave := domain.Wave{
		ID:          "w1",
		ClusterName: "auth",
		Title:       "Auth Cluster Wave",
	}
	errs := []string{"permission denied: /etc/config", "permission denied: /etc/config"}
	reason := "repeated structural errors detected"

	// when
	body := session.StallEscalationBody(wave, errs, reason)

	// then
	if !strings.Contains(body, wave.Title) {
		t.Errorf("StallEscalationBody: expected body to contain wave title %q", wave.Title)
	}
	if !strings.Contains(body, reason) {
		t.Errorf("StallEscalationBody: expected body to contain reason %q", reason)
	}
	if !strings.Contains(body, "permission denied") {
		t.Error("StallEscalationBody: expected body to contain error details")
	}
}

func TestStructuralErrors_FiltersCorrectly(t *testing.T) {
	// given: a mix of structural and transient errors
	allErrors := []string{
		"permission denied: /etc/foo",
		"connection reset by peer",
		"no such file or directory",
		"context deadline exceeded",
	}

	// when
	structural := session.StructuralErrors(allErrors)

	// then
	if len(structural) != 2 {
		t.Errorf("StructuralErrors: expected 2 structural errors, got %d: %v", len(structural), structural)
	}
	for _, e := range structural {
		kind := domain.ClassifyError(e)
		if kind != domain.ErrorKindStructural {
			t.Errorf("StructuralErrors: expected all returned errors to be structural, got %q", e)
		}
	}
}

func TestComposeStallEscalation_StagesOutbox(t *testing.T) {
	// given
	ctx := context.Background()
	dir := t.TempDir()
	store := testOutboxStore(t, dir)
	wave := domain.Wave{
		ID:          "w2",
		ClusterName: "infra",
		Title:       "Infra Wave",
	}
	errs := []string{"permission denied: /etc/init"}
	reason := "stalled after 3 structural errors"

	// when
	err := session.ComposeStallEscalation(ctx, store, wave, errs, reason)

	// then
	if err != nil {
		t.Fatalf("ComposeStallEscalation: unexpected error: %v", err)
	}
}
