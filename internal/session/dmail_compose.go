package session

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/harness"
	"github.com/hironow/sightjack/internal/usecase/port"
)

// ErrSpecNoImplementationSteps indicates that a wave had no implementation-oriented
// actions after filtering issue-management types, so no spec D-Mail was generated.
var ErrSpecNoImplementationSteps = errors.New("spec: no implementation steps after filtering")

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
	return ComposeReportWithMetadata(ctx, store, wave, result, domain.CorrectionMetadata{})
}

func ComposeReportWithMetadata(ctx context.Context, store port.OutboxStore, wave domain.Wave, result *domain.WaveApplyResult, meta domain.CorrectionMetadata) error {
	key := domain.WaveKey(wave)
	mail := &domain.DMail{
		Name:          DMailName("report", key),
		Kind:          domain.KindReport,
		Description:   fmt.Sprintf("Wave %s completed", key),
		SchemaVersion: domain.DMailSchemaVersion,
		Issues:        WaveIssueIDs(wave),
		Body:          ReportBody(wave, result),
	}
	if meta.SchemaVersion != "" {
		mail.Metadata = meta.Apply(mail.Metadata)
	}
	mail.Metadata = currentProviderState().ApplyMetadata(mail.Metadata)
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
// Uses domain.KindReport because sightjack's sendable contract only produces specification and report.
func ComposeFeedback(ctx context.Context, store port.OutboxStore, wave domain.Wave, result *domain.WaveApplyResult) error {
	return ComposeFeedbackWithMetadata(ctx, store, wave, result, domain.CorrectionMetadata{})
}

func ComposeFeedbackWithMetadata(ctx context.Context, store port.OutboxStore, wave domain.Wave, result *domain.WaveApplyResult, meta domain.CorrectionMetadata) error {
	key := domain.WaveKey(wave)
	mail := &domain.DMail{
		Name:          DMailName("feedback", key),
		Kind:          domain.KindReport,
		Description:   fmt.Sprintf("Wave %s report for amadeus", key),
		SchemaVersion: domain.DMailSchemaVersion,
		Issues:        WaveIssueIDs(wave),
		Body:          FeedbackBody(wave, result),
	}
	if meta.SchemaVersion != "" {
		mail.Metadata = meta.Apply(mail.Metadata)
	}
	mail.Metadata = currentProviderState().ApplyMetadata(mail.Metadata)
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
//
// In wave mode, the body is rendered as a Rival Contract v1 specification
// and the D-Mail metadata carries contract_schema, contract_id,
// contract_revision, and supersedes per refs/plans/2026-05-03-rival-contract-v1.md.
// Legacy non-wave callers retain the existing SpecificationBody output.
func ComposeSpecification(ctx context.Context, store port.OutboxStore, wave domain.Wave, mode ...domain.TrackingMode) error {
	key := domain.WaveKey(wave)
	mail := &domain.DMail{
		Name:          DMailName("spec", key),
		Kind:          domain.KindSpecification,
		Description:   wave.Title,
		SchemaVersion: domain.DMailSchemaVersion,
		Issues:        WaveIssueIDs(wave),
		Body:          SpecificationBody(wave),
	}

	// Wave mode: attach WaveReference with implementation actions as steps
	// and render Rival Contract v1 body + metadata.
	if len(mode) > 0 && mode[0].IsWave() {
		var implActs, issueMgmtActs []domain.WaveAction
		ref := &domain.WaveReference{ID: key}
		for _, action := range wave.Actions {
			if issueManagementTypes[action.Type] {
				issueMgmtActs = append(issueMgmtActs, action)
				continue // already applied by sightjack
			}
			implActs = append(implActs, action)
			ref.Steps = append(ref.Steps, domain.WaveStepDef{
				ID:          action.IssueID,
				Title:       action.Description,
				Description: action.Detail,
			})
		}
		// No implementation steps → signal to caller (not a failure)
		if len(ref.Steps) == 0 {
			return ErrSpecNoImplementationSteps
		}
		mail.Wave = ref

		// Replace the legacy action-list body with a Rival Contract v1 body.
		mail.Body = harness.RenderRivalContract(harness.RivalContractInput{
			Title:              wave.Title,
			Description:        wave.Description,
			ClusterName:        wave.ClusterName,
			IssueIDs:           mail.Issues,
			ImplementationActs: implActs,
			IssueMgmtActs:      issueMgmtActs,
		})

		// Attach Rival Contract v1 metadata. contract_id MUST be stable
		// across revisions and MUST NOT use the D-Mail name (per plan).
		contractID, err := harness.DeriveContractID(wave.ID, mail.Issues, wave.ClusterName)
		if err != nil {
			return fmt.Errorf("compose specification: derive contract id: %w", err)
		}
		if mail.Metadata == nil {
			mail.Metadata = make(map[string]string, 4)
		}
		mail.Metadata["contract_schema"] = harness.SchemaRivalContractV1
		mail.Metadata["contract_id"] = contractID
		mail.Metadata["contract_revision"] = strconv.Itoa(1)
		mail.Metadata["supersedes"] = ""
	}

	return ComposeDMail(ctx, store, mail)
}
