package sightjack

import (
	"context"
	"sort"
)

// Handoff defines the integration contract for downstream execution agents (v1.0).
// Implementations receive ready issue IDs and execute them via Claude Code agents.
type Handoff interface {
	// HandoffReady delivers a batch of ready issue IDs to a downstream agent
	// for autonomous execution. Returns an error if the handoff fails.
	HandoffReady(ctx context.Context, issueIDs []string) error

	// ReportIssue reports a finding (e.g. blocker, question, anomaly) back
	// to the orchestrator for a specific issue during execution.
	ReportIssue(ctx context.Context, issueID string, finding string) error
}

// HandoffResult tracks the outcome of a handoff for a single issue.
type HandoffResult struct {
	IssueID string // Linear issue identifier
	Status  string // "success", "failed", "skipped"
	Error   string // non-empty when Status is "failed"
}

// ReadyIssueIDs returns issue IDs where ALL waves targeting them are completed.
// An issue is ready when every wave containing that issue has status "completed".
// Results are sorted for deterministic output.
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
	sort.Strings(ready)
	return ready
}
