package sightjack

// ReadyIssueIDsAssuming returns issue IDs where ALL waves targeting them are completed,
// treating the given wave as already completed regardless of its actual status.
// This is used before wave apply to include issues that will become ready after the current wave.
func ReadyIssueIDsAssuming(waves []Wave, assuming Wave) []string {
	assumingKey := WaveKey(assuming)
	issueWaves := make(map[string][]string)
	for _, w := range waves {
		status := w.Status
		if WaveKey(w) == assumingKey {
			status = "completed"
		}
		for _, a := range w.Actions {
			issueWaves[a.IssueID] = append(issueWaves[a.IssueID], status)
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
	return ready
}

// ReadyIssueIDs returns issue IDs where ALL waves targeting them are completed.
// An issue is ready when every wave containing that issue has status "completed".
func ReadyIssueIDs(waves []Wave) []string {
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
	return ready
}
