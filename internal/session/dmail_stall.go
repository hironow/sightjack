package session

import (
	"context"
	"fmt"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/usecase/port"
)

// DMailStallEscalation is the d-mail kind for stall escalation messages.
// Sent when a wave is detected as stalled due to repeated structural errors.
const DMailStallEscalation DMailKind = "stall-escalation"

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
		Body:          domain.StallEscalationBody(wave, errors, reason),
	}
	return ComposeDMail(ctx, store, mail)
}
