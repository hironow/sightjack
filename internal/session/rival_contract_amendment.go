// Package session — rival_contract_amendment.go: Rival Contract v1
// amendment loop helpers (Phase 5).
//
// The amendment loop turns amadeus design-feedback D-Mails (with a
// `## Contract Amendments` section) into concrete amended specification
// D-Mails. Two responsibilities live here:
//
//  1. ExtractContractAmendments parses the canonical bullet grammar
//     emitted by amadeus's Phase 3 corrective composer:
//
//     ^- Contract: `<contract_id>`( \(revision <N>\))?$
//     ^- Amend <Section>: <Suggestion>( \(rationale: <Rationale>\))?$
//
//     The extractor is deterministic, never invokes an LLM, and
//     silently skips bullets that do not match. Bullets with a section
//     of "(unspecified)" are passed through unchanged so the nextgen
//     LLM call can infer the target section instead of dropping the
//     amendment.
//
//  2. ComposeAmendedSpecification produces a NEW Rival Contract v1
//     specification D-Mail with revision = previous + 1, supersedes set
//     to the previous current D-Mail name, and a body that cites the
//     originating feedback D-Mail by name. The previous D-Mail file is
//     never modified — append-only by construction.
//
// Refs: refs/plans/2026-05-03-rival-contract-v1.md (Phase 5 PR).
package session

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/harness"
	"github.com/hironow/sightjack/internal/usecase/port"
)

// RivalContractAmendment is one parsed amendment bullet from a
// `## Contract Amendments` section. Section may be a canonical Rival
// Contract heading (Intent, Domain, Decisions, Steps, Boundaries,
// Evidence) or the literal "(unspecified)" placeholder when amadeus
// could not infer a target section. Suggestion is the prose payload;
// Rationale is empty when the bullet has no `(rationale: ...)` clause.
type RivalContractAmendment struct { // nosemgrep: structure.multiple-exported-structs-go -- Rival Contract v1 amendment-loop type co-located with extractor + compose path; cohesive Phase 5 set [permanent]
	ContractID string
	Section    string
	Suggestion string
	Rationale  string
}

// amendmentsHeading matches the canonical section heading emitted by
// amadeus and is the only entry point for the deterministic parse.
const amendmentsHeading = "## Contract Amendments"

// contractIDBulletRE matches the leading "- Contract: `<id>`" identity
// bullet that prefixes a `## Contract Amendments` block. The optional
// "(revision N)" suffix is parsed but not retained (revision lives in
// metadata, not in amendment payloads).
var contractIDBulletRE = regexp.MustCompile(`^-\s+Contract:\s*` + "`" + `([^` + "`" + `]+)` + "`" + `(?:\s+\(revision\s+\d+\))?\s*$`)

// amendBulletRE matches a single "- Amend <Section>: <Suggestion>"
// bullet, with an optional " (rationale: <Rationale>)" suffix. The
// section group captures everything between "Amend " and the first
// colon, including the literal "(unspecified)" placeholder used when
// amadeus could not infer a target section.
var amendBulletRE = regexp.MustCompile(`^-\s+Amend\s+(\S(?:.*?\S)?)\s*:\s*(.*?)(?:\s+\(rationale:\s*(.*?)\))?\s*$`)

// ExtractContractAmendments parses the `## Contract Amendments` block
// of a feedback body and returns one RivalContractAmendment per
// well-formed bullet, in document order. The function returns an empty
// slice (and nil error) when the section is absent so callers can
// fall back to legacy nextgen behavior unchanged.
//
// Bullets that do not match the canonical grammar are skipped silently
// — the amendment loop is best-effort by design and must not panic on
// noisy or partially-formed feedback bodies.
func ExtractContractAmendments(feedbackBody string) ([]RivalContractAmendment, error) {
	section, ok := sliceContractAmendmentsSection(feedbackBody)
	if !ok {
		return nil, nil
	}

	contractID := ""
	var out []RivalContractAmendment
	for _, raw := range strings.Split(section, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		// Identity bullet (one expected, but tolerate repeats).
		if m := contractIDBulletRE.FindStringSubmatch(line); m != nil {
			contractID = strings.TrimSpace(m[1])
			continue
		}
		m := amendBulletRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		amendSection := strings.TrimSpace(m[1])
		if amendSection == "" {
			continue
		}
		suggestion := strings.TrimSpace(m[2])
		if suggestion == "" {
			continue
		}
		rationale := ""
		if len(m) > 3 {
			rationale = strings.TrimSpace(m[3])
		}
		out = append(out, RivalContractAmendment{
			ContractID: contractID,
			Section:    amendSection,
			Suggestion: suggestion,
			Rationale:  rationale,
		})
	}
	return out, nil
}

// sliceContractAmendmentsSection returns the contiguous text under the
// canonical `## Contract Amendments` heading up to (but not including)
// the next `## ` heading or end-of-body. The boolean is false when the
// heading is absent.
func sliceContractAmendmentsSection(body string) (string, bool) {
	lines := strings.Split(body, "\n")
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == amendmentsHeading {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return "", false
	}
	end := len(lines)
	for i := start; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "## ") {
			end = i
			break
		}
	}
	return strings.Join(lines[start:end], "\n"), true
}

// ComposeAmendedSpecification produces a NEW Rival Contract v1
// specification D-Mail that supersedes `previousName` at revision
// `previous.Revision + 1`. The previous D-Mail file is left untouched
// — append-only is a hard invariant of the contract loop.
//
// The amended D-Mail uses the same wave-mode renderer as
// ComposeSpecification (so all six canonical sections remain), then
// applies three lineage overrides:
//
//   - contract_id is preserved from `previous` (wave id is stable across
//     revisions per plan §"Contract Authoring Rules").
//   - contract_revision is set to previous.Revision + 1.
//   - supersedes is set to previousName so the lineage projection in
//     amadeus can verify it on the next archive scan.
//
// A short `## Amendment Lineage` block is appended to the body that
// cites the originating feedback D-Mail by name and lists the
// amendments that drove the revision. The canonical six Rival Contract
// section headings are untouched, so the body still parses with
// ParseRivalContractBody.
//
// The returned *domain.DMail is the staged-and-flushed representation.
// Callers that only need the side effect can discard the value.
func ComposeAmendedSpecification(
	ctx context.Context,
	store port.OutboxStore,
	wave domain.Wave,
	previous harness.RivalContractMetadata,
	previousName string,
	amendments []RivalContractAmendment,
	feedbackName string,
	mode ...domain.TrackingMode,
) (*domain.DMail, error) {
	if previousName == "" {
		return nil, fmt.Errorf("compose amended specification: previousName is required")
	}
	if previous.Revision <= 0 {
		return nil, fmt.Errorf("compose amended specification: previous.Revision must be positive, got %d", previous.Revision)
	}
	if len(mode) == 0 || !mode[0].IsWave() {
		return nil, fmt.Errorf("compose amended specification: wave mode is required (Rival Contract v1 only flows through wave-mode specs)")
	}

	mail, err := buildAmendedSpecMail(wave, previous, previousName, amendments, feedbackName)
	if err != nil {
		return nil, err
	}
	if err := ComposeDMail(ctx, store, mail); err != nil {
		return nil, fmt.Errorf("compose amended specification: %w", err)
	}
	return mail, nil
}

// buildAmendedSpecMail assembles the in-memory amended specification
// D-Mail without touching the outbox store. Split out for testability
// and to keep ComposeAmendedSpecification a thin wrapper.
//
// The amended D-Mail name embeds the new revision (e.g.
// `sj-spec-<key>-rev2_<uuid>`) so the on-disk file does NOT collide
// with the previous revision's file even when the deterministic test
// UUID is fixed. Append-only file semantics are an explicit Phase 5
// invariant.
func buildAmendedSpecMail(wave domain.Wave, previous harness.RivalContractMetadata, previousName string, amendments []RivalContractAmendment, feedbackName string) (*domain.DMail, error) {
	key := domain.WaveKey(wave)
	newRevision := previous.Revision + 1
	revisionedKey := key + "-rev" + strconv.Itoa(newRevision)
	issues := WaveIssueIDs(wave)

	var implActs, issueMgmtActs []domain.WaveAction
	ref := &domain.WaveReference{ID: key}
	for _, action := range wave.Actions {
		if issueManagementTypes[action.Type] {
			issueMgmtActs = append(issueMgmtActs, action)
			continue
		}
		implActs = append(implActs, action)
		ref.Steps = append(ref.Steps, domain.WaveStepDef{
			ID:          action.IssueID,
			Title:       action.Description,
			Description: action.Detail,
		})
	}
	if len(ref.Steps) == 0 {
		return nil, ErrSpecNoImplementationSteps
	}

	body := harness.RenderRivalContract(harness.RivalContractInput{
		Title:              wave.Title,
		Description:        wave.Description,
		ClusterName:        wave.ClusterName,
		IssueIDs:           issues,
		ImplementationActs: implActs,
		IssueMgmtActs:      issueMgmtActs,
	})
	body = appendAmendmentCitation(body, previous, previousName, feedbackName, amendments)

	contractID := previous.ID
	if contractID == "" {
		var err error
		contractID, err = harness.DeriveContractID(wave.ID, issues, wave.ClusterName)
		if err != nil {
			return nil, fmt.Errorf("compose amended specification: derive contract id: %w", err)
		}
	}

	mail := &domain.DMail{
		Name:          DMailName("spec", revisionedKey),
		Kind:          domain.KindSpecification,
		Description:   wave.Title,
		SchemaVersion: domain.DMailSchemaVersion,
		Issues:        issues,
		Wave:          ref,
		Body:          body,
		Metadata: map[string]string{
			"contract_schema":   harness.SchemaRivalContractV1,
			"contract_id":       contractID,
			"contract_revision": strconv.Itoa(newRevision),
			"supersedes":        previousName,
		},
	}
	mail.Metadata = currentProviderState().ApplyMetadata(mail.Metadata)
	return mail, nil
}

// renderAmendmentSection returns the canonical section label for an
// amendment bullet. It mirrors the contractAmendmentSection helper in
// amadeus's corrective composer so the round-trip "amadeus emits →
// sightjack re-renders" is byte-stable for callers that do not provide
// a section name. Emitting `(unspecified)` instead of an empty string
// keeps the Markdown bullet self-describing for human readers and the
// nextgen LLM.
func renderAmendmentSection(section string) string {
	switch strings.TrimSpace(section) {
	case "":
		return "(unspecified)"
	default:
		return section
	}
}

// appendAmendmentCitation appends a short Markdown block to a Rival
// Contract v1 body that records the amendment lineage:
//
//	## Amendment Lineage
//	- Amended in response to feedback: <feedbackName>
//	- Supersedes: <previousName> (revision <previous.Revision>)
//	- Amend <Section>: <Suggestion> (rationale: ...)
//
// The block is appended verbatim — the canonical six Rival Contract
// section headings remain untouched, so ParseRivalContractBody still
// returns ok=true after the append.
func appendAmendmentCitation(body string, previous harness.RivalContractMetadata, previousName, feedbackName string, amendments []RivalContractAmendment) string {
	var b strings.Builder
	b.WriteString(strings.TrimRight(body, "\n"))
	b.WriteString("\n\n## Amendment Lineage\n\n")
	if feedbackName != "" {
		fmt.Fprintf(&b, "- Amended in response to feedback: %s\n", feedbackName)
	}
	if previousName != "" {
		if previous.Revision > 0 {
			fmt.Fprintf(&b, "- Supersedes: %s (revision %d)\n", previousName, previous.Revision)
		} else {
			fmt.Fprintf(&b, "- Supersedes: %s\n", previousName)
		}
	}
	for _, a := range amendments {
		// Section comes from ExtractContractAmendments, which already
		// drops bullets with empty section text and preserves the
		// literal "(unspecified)" placeholder amadeus emits when it
		// could not infer a target. No fallback is needed here; the
		// session layer relies on producer invariants per the
		// session-no-empty-string-fallback semgrep rule rationale.
		if a.Rationale != "" {
			fmt.Fprintf(&b, "- Amend %s: %s (rationale: %s)\n", renderAmendmentSection(a.Section), a.Suggestion, a.Rationale)
		} else {
			fmt.Fprintf(&b, "- Amend %s: %s\n", renderAmendmentSection(a.Section), a.Suggestion)
		}
	}
	b.WriteString("\n")
	return b.String()
}
