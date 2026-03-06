package session

import (
	"github.com/hironow/sightjack/internal/domain"
)

// ReadyIssueIDs returns issue IDs where ALL waves targeting them are completed.
// Delegates to domain.ReadyIssueIDs.
func ReadyIssueIDs(waves []domain.Wave) []string {
	return domain.ReadyIssueIDs(waves)
}
