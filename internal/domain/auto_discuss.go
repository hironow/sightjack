package domain

import (
	"fmt"
	"strings"
)

// AutoDiscussRound captures a single round of the auto-discuss debate.
type AutoDiscussRound struct {
	Round   int    `json:"round"`
	Speaker string `json:"speaker"` // "architect" or "devils_advocate"
	Content string `json:"content"`
}

// AutoDiscussResult holds the full auto-discuss debate outcome.
type AutoDiscussResult struct {
	Rounds     []AutoDiscussRound `json:"rounds"`
	OpenIssues []string           `json:"open_issues"`
	Summary    string             `json:"summary"`
}

// ToArchitectResponse converts the debate result into an ArchitectResponse
// so it can be passed to RunScribeADR unchanged.
func (r AutoDiscussResult) ToArchitectResponse() *ArchitectResponse {
	var analysis strings.Builder
	if len(r.Rounds) == 0 {
		analysis.WriteString("Auto-discuss completed with no debate rounds.")
	}
	for _, round := range r.Rounds {
		fmt.Fprintf(&analysis, "[%s round %d]: %s\n\n", round.Speaker, round.Round, round.Content)
	}

	var reasoning strings.Builder
	if r.Summary == "" {
		reasoning.WriteString("Auto-discuss completed with no summary.")
	} else {
		reasoning.WriteString(r.Summary)
	}
	if len(r.OpenIssues) > 0 {
		reasoning.WriteString("\n\nOpen issues:\n")
		for _, issue := range r.OpenIssues {
			fmt.Fprintf(&reasoning, "- %s\n", issue)
		}
	}

	decision := r.Summary
	if decision == "" {
		decision = "Auto-discuss completed with no summary."
	}

	return &ArchitectResponse{
		Analysis:     analysis.String(),
		Reasoning:    reasoning.String(),
		Decision:     decision,
		ModifiedWave: nil,
	}
}
