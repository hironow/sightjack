package session

import (
	"context"
	"fmt"
	"strings"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/usecase/port"
)

// DMailStallEscalation is the d-mail kind for stall escalation messages.
// Sent when a wave is detected as stalled due to repeated structural errors.
const DMailStallEscalation DMailKind = "stall-escalation"

// StructuralErrors filters errMsg slice, returning only those classified as
// structural by domain.ClassifyError.
func StructuralErrors(errors []string) []string {
	var result []string
	for _, e := range errors {
		if domain.ClassifyError(e) == domain.ErrorKindStructural {
			result = append(result, e)
		}
	}
	return result
}

// StallEscalationBody formats a d-mail body for a stall escalation message.
// It includes the wave title, the escalation reason, and the list of structural errors.
func StallEscalationBody(wave domain.Wave, errors []string, reason string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Stall Escalation: %s\n\n", wave.Title)
	fmt.Fprintf(&b, "**Wave:** %s\n", domain.WaveKey(wave))
	fmt.Fprintf(&b, "**Reason:** %s\n\n", reason)
	if len(errors) > 0 {
		fmt.Fprintf(&b, "## Structural Errors\n\n")
		for _, e := range errors {
			fmt.Fprintf(&b, "- %s\n", e)
		}
	}
	return b.String()
}

// ComposeStallEscalation stages a stall-escalation D-Mail in the outbox.
// Called when a wave is detected as stalled due to repeated structural errors.
func ComposeStallEscalation(ctx context.Context, store port.OutboxStore, wave domain.Wave, errors []string, reason string) error {
	key := domain.WaveKey(wave)
	mail := &DMail{
		Name:          DMailName("stall", key),
		Kind:          DMailStallEscalation,
		Description:   fmt.Sprintf("Wave %s stalled: %s", key, reason),
		SchemaVersion: "1",
		Issues:        WaveIssueIDs(wave),
		Severity:      "high",
		Action:        "escalate",
		Body:          StallEscalationBody(wave, errors, reason),
	}
	return ComposeDMail(ctx, store, mail)
}
