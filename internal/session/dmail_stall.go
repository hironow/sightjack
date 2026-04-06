package session

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/usecase/port"
)

// DMailStallEscalation is the d-mail kind for stall escalation messages.
// Sent when a wave is detected as stalled due to repeated structural errors.
const DMailStallEscalation DMailKind = "stall-escalation"

// ComposeStallEscalation stages a stall-escalation D-Mail in the outbox.
// Called when a wave is detected as stalled due to repeated structural errors.
// Metadata includes wave_id, cluster_name, error_fingerprint, failure_count, detected_at
// as required by SPEC-001.
func ComposeStallEscalation(ctx context.Context, store port.OutboxStore, wave domain.Wave, errors []string, reason, fingerprint string, failureCount int) error {
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
		Metadata: map[string]string{
			"wave_id":           wave.ID,
			"cluster_name":     wave.ClusterName,
			"error_fingerprint": fingerprint,
			"failure_count":     strconv.Itoa(failureCount),
			"detected_at":      time.Now().UTC().Format(time.RFC3339),
		},
	}
	return ComposeDMail(ctx, store, mail)
}
