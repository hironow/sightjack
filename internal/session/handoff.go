package session

import (
	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/harness"
)

// ReadyIssueIDs returns issue IDs where ALL waves targeting them are completed.
// Delegates to harness.ReadyIssueIDs.
func ReadyIssueIDs(waves []domain.Wave) []string {
	return harness.ReadyIssueIDs(waves)
}
