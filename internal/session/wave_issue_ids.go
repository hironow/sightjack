package session

import (
	"sort"

	"github.com/hironow/sightjack/internal/domain"
)

// WaveIssueIDs returns the sorted, de-duplicated set of issue IDs referenced by
// a wave's actions. Used by D-Mail stall detection and improvement metadata.
func WaveIssueIDs(wave domain.Wave) []string {
	seen := make(map[string]bool)
	for _, a := range wave.Actions {
		if a.IssueID != "" {
			seen[a.IssueID] = true
		}
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
