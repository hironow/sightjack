package domain

// white-box-reason: tests unexported StructuralErrors and StallEscalationBody pure functions

import (
	"strings"
	"testing"
)

func TestStructuralErrors_FiltersCorrectly(t *testing.T) {
	// given: a mix of structural and transient errors
	allErrors := []string{
		"permission denied: /etc/foo",
		"connection reset by peer",
		"no such file or directory",
		"context deadline exceeded",
	}

	// when
	structural := StructuralErrors(allErrors)

	// then
	if len(structural) != 2 {
		t.Errorf("StructuralErrors: expected 2 structural errors, got %d: %v", len(structural), structural)
	}
	for _, e := range structural {
		kind := ClassifyError(e)
		if kind != ErrorKindStructural {
			t.Errorf("StructuralErrors: expected all returned errors to be structural, got %q", e)
		}
	}
}

func TestStallEscalationBody_ContainsWaveAndReason(t *testing.T) {
	// given
	wave := Wave{
		ID:          "w1",
		ClusterName: "auth",
		Title:       "Auth Cluster Wave",
	}
	errs := []string{"permission denied: /etc/config", "permission denied: /etc/config"}
	reason := "repeated structural errors detected"

	// when
	body := StallEscalationBody(wave, errs, reason)

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
