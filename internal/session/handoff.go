package session

import (
	"sort"

	sightjack "github.com/hironow/sightjack"
)

// ReadyIssueIDs returns issue IDs where ALL waves targeting them are completed.
// An issue is ready when every wave containing that issue has status "completed".
// Results are sorted for deterministic output.
func ReadyIssueIDs(waves []sightjack.Wave) []string {
	// Track all waves per issue
	issueWaves := make(map[string][]string) // issueID -> []waveStatus
	for _, w := range waves {
		for _, a := range w.Actions {
			issueWaves[a.IssueID] = append(issueWaves[a.IssueID], w.Status)
		}
	}

	var ready []string
	for issueID, statuses := range issueWaves {
		allCompleted := true
		for _, s := range statuses {
			if s != "completed" {
				allCompleted = false
				break
			}
		}
		if allCompleted {
			ready = append(ready, issueID)
		}
	}
	sort.Strings(ready)
	return ready
}
