package session

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/usecase/port"
)

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

// SpecificationBody formats wave actions as Markdown body for a specification d-mail.
func SpecificationBody(wave domain.Wave) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", wave.Title)
	if wave.Description != "" {
		fmt.Fprintf(&b, "%s\n\n", wave.Description)
	}
	fmt.Fprintf(&b, "## Actions\n\n")
	for _, a := range wave.Actions {
		fmt.Fprintf(&b, "- [%s] %s: %s\n", a.Type, a.IssueID, a.Description)
	}
	return b.String()
}

// ReportBody formats wave apply results as Markdown body for a report d-mail.
func ReportBody(wave domain.Wave, result *domain.WaveApplyResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Wave Completed: %s\n\n", wave.Title)
	fmt.Fprintf(&b, "Applied %d action(s).\n\n", result.Applied)
	if len(result.Errors) > 0 {
		fmt.Fprintf(&b, "## Errors\n\n")
		for _, e := range result.Errors {
			fmt.Fprintf(&b, "- %s\n", e)
		}
		b.WriteString("\n")
	}
	if len(result.Ripples) > 0 {
		fmt.Fprintf(&b, "## Ripple Effects\n\n")
		for _, r := range result.Ripples {
			fmt.Fprintf(&b, "- [%s] %s\n", r.ClusterName, r.Description)
		}
	}
	return b.String()
}

// ComposeReport creates and sends a report d-mail for a completed wave.
func ComposeReport(ctx context.Context, store port.OutboxStore, wave domain.Wave, result *domain.WaveApplyResult) error {
	key := domain.WaveKey(wave)
	mail := &DMail{
		Name:          DMailName("report", key),
		Kind:          DMailReport,
		Description:   fmt.Sprintf("Wave %s completed", key),
		SchemaVersion: "1",
		Issues:        WaveIssueIDs(wave),
		Body:          ReportBody(wave, result),
	}
	return ComposeDMail(ctx, store, mail)
}

// FeedbackBody formats wave apply results as Markdown body for a feedback d-mail.
// Distinct from ReportBody: uses "Wave Feedback" heading to differentiate the
// sightjack → amadeus feedback loop (O2) from the standard report d-mail.
func FeedbackBody(wave domain.Wave, result *domain.WaveApplyResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Wave Feedback: %s\n\n", wave.Title)
	fmt.Fprintf(&b, "Applied %d action(s).\n\n", result.Applied)
	if len(result.Errors) > 0 {
		fmt.Fprintf(&b, "## Errors\n\n")
		for _, e := range result.Errors {
			fmt.Fprintf(&b, "- %s\n", e)
		}
		b.WriteString("\n")
	}
	if len(result.Ripples) > 0 {
		fmt.Fprintf(&b, "## Ripple Effects\n\n")
		for _, r := range result.Ripples {
			fmt.Fprintf(&b, "- [%s] %s\n", r.ClusterName, r.Description)
		}
	}
	return b.String()
}

// ComposeFeedback stages a report D-Mail for amadeus consumption.
// Called after successful wave apply to complete the sightjack → amadeus feedback loop (O2).
// Uses DMailReport kind because sightjack's sendable contract only produces specification and report.
func ComposeFeedback(ctx context.Context, store port.OutboxStore, wave domain.Wave, result *domain.WaveApplyResult) error {
	key := domain.WaveKey(wave)
	mail := &DMail{
		Name:          DMailName("feedback", key),
		Kind:          DMailReport,
		Description:   fmt.Sprintf("Wave %s report for amadeus", key),
		SchemaVersion: "1",
		Issues:        WaveIssueIDs(wave),
		Body:          FeedbackBody(wave, result),
	}
	return ComposeDMail(ctx, store, mail)
}

// issueManagementTypes lists action types that are handled by sightjack
// during wave apply. These must not be forwarded to paintress via spec D-Mail.
var issueManagementTypes = map[string]bool{
	"add_dod":            true,
	"add_dependency":     true,
	"add_label":          true,
	"update_description": true,
	"create":             true,
	"cancel":             true,
}

// ComposeSpecification creates and sends a specification d-mail for an approved wave.
// In wave mode, issue management actions are filtered out — only implementation-oriented
// actions (implement, fix, verify, etc.) are included as steps. If no implementation
// actions remain, the spec D-Mail is not generated.
func ComposeSpecification(ctx context.Context, store port.OutboxStore, wave domain.Wave, mode ...domain.TrackingMode) error {
	key := domain.WaveKey(wave)
	mail := &DMail{
		Name:          DMailName("spec", key),
		Kind:          DMailSpecification,
		Description:   wave.Title,
		SchemaVersion: "1",
		Issues:        WaveIssueIDs(wave),
		Body:          SpecificationBody(wave),
	}

	// Wave mode: attach WaveReference with implementation actions as steps
	if len(mode) > 0 && mode[0].IsWave() {
		ref := &domain.WaveReference{ID: key}
		for _, action := range wave.Actions {
			if issueManagementTypes[action.Type] {
				continue // already applied by sightjack
			}
			ref.Steps = append(ref.Steps, domain.WaveStepDef{
				ID:          action.IssueID,
				Title:       action.Description,
				Description: action.Detail,
			})
		}
		// No implementation steps → skip spec D-Mail entirely
		if len(ref.Steps) == 0 {
			return nil
		}
		mail.Wave = ref
	}

	return ComposeDMail(ctx, store, mail)
}
