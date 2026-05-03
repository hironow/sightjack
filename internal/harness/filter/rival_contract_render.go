// Package filter rival_contract_render.go: Rival Contract v1 renderer.
//
// Pure rendering utility that produces the Markdown body for a
// `kind: specification` D-Mail in Rival Contract v1 format. The renderer
// performs no I/O, never invokes an LLM, and is the inverse of the parser
// in rival_contract.go: any output of RenderRivalContract that uses
// non-empty inputs MUST round-trip through ParseRivalContractBody.
//
// Refs: refs/plans/2026-05-03-rival-contract-v1.md (Phase 1)
package filter

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hironow/sightjack/internal/domain"
)

// RivalContractInput carries the data needed to render a Rival Contract v1
// specification body. The compose layer is responsible for splitting wave
// actions into implementation vs issue-management buckets before calling
// the renderer.
type RivalContractInput struct { // nosemgrep: structure.multiple-exported-structs-go -- Rival Contract v1 type family co-located with RivalContract [permanent]
	// Title is the wave title; it becomes `# Contract: <Title>`.
	Title string
	// Description is free-form prose; it seeds the Intent section.
	Description string
	// ClusterName is the originating cluster id; used in the Domain section.
	ClusterName string
	// IssueIDs is the deduplicated list of upstream issue identifiers tied
	// to this wave. Used in the Domain section.
	IssueIDs []string
	// ImplementationActs are the wave actions that translate to executable
	// implementation steps (i.e. issue-management types are already
	// filtered out by the caller). Rendered in the Steps section.
	ImplementationActs []domain.WaveAction
	// IssueMgmtActs are the wave actions that sightjack handles directly
	// (add_dod, add_dependency, ...). They are surfaced in the Boundaries
	// section so consumers can see what sightjack already applied.
	IssueMgmtActs []domain.WaveAction
	// DecisionsSummary is an optional pre-formatted prose summary of any
	// design discussion that produced this wave. When empty, the renderer
	// emits the canonical placeholder text.
	DecisionsSummary string
	// AdditionalBoundaries lists extra free-form boundary statements (for
	// example DoD constraints) the caller wants to attach. Each entry is
	// rendered as a single bullet under Boundaries.
	AdditionalBoundaries []string
	// AdditionalEvidence lists extra free-form evidence prose bullets.
	// Each entry is rendered as a single bullet under Evidence in addition
	// to the canonical machine-readable grammar bullets.
	AdditionalEvidence []string
	// DomainStyle is the OPTIONAL Rival Contract v1.1 vocabulary tag the
	// producer chose for this contract (one of "event-sourced", "generic",
	// "mixed"). When non-empty, the renderer will emit a
	// `domain_style: <value>` line via RenderRivalContractMetadata. An
	// empty value yields output bit-identical to the v1 renderer; the
	// producer must NEVER set this from a parser-time inference path —
	// the parser refuses to infer and the renderer refuses to default.
	DomainStyle string
}

// decisionsPlaceholder is rendered when the caller did not supply a
// discussion summary. The exact wording is referenced by Phase 1 tests.
const decisionsPlaceholder = "No explicit design decision recorded."

// RenderRivalContract renders a Rival Contract v1 specification body.
//
// The output always contains, in order:
//
//	# Contract: <title>
//	## Intent
//	## Domain
//	## Decisions
//	## Steps
//	## Boundaries
//	## Evidence
//
// All paths in the body are repository-relative; absolute paths are never
// emitted. Issue-management actions are surfaced under Boundaries (they
// are NOT executable steps), and Steps lists only ImplementationActs.
func RenderRivalContract(in RivalContractInput) string {
	var b strings.Builder

	title := strings.TrimSpace(in.Title)
	if title == "" {
		title = "Untitled Wave"
	}
	fmt.Fprintf(&b, "# Contract: %s\n\n", title)

	writeSection(&b, "Intent", renderIntentSection(in))
	writeSection(&b, "Domain", renderDomainSection(in))
	writeSection(&b, "Decisions", renderDecisionsSection(in))
	writeSection(&b, "Steps", renderStepsSection(in))
	writeSection(&b, "Boundaries", renderBoundariesSection(in))
	writeSection(&b, "Evidence", renderEvidenceSection(in))

	return b.String()
}

// writeSection writes a `## <heading>` section followed by the trimmed
// body and a trailing blank line. Empty bodies are normalized to a single
// dash bullet so the parser still recognizes the section as non-empty.
func writeSection(b *strings.Builder, heading, content string) {
	fmt.Fprintf(b, "## %s\n\n", heading)
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		trimmed = "- (none)"
	}
	b.WriteString(trimmed)
	b.WriteString("\n\n")
}

// renderIntentSection produces a short prose plus bullet list of why this
// wave exists.
func renderIntentSection(in RivalContractInput) string {
	var b strings.Builder
	if desc := strings.TrimSpace(in.Description); desc != "" {
		fmt.Fprintf(&b, "%s\n\n", desc)
	}
	fmt.Fprintf(&b, "- Deliver wave %q so the implementing tool can act on a single self-contained contract.\n", titleOrDefault(in))
	fmt.Fprintf(&b, "- Success means every implementation step under Steps is executed and its acceptance signal is observable in Evidence.\n")
	return b.String()
}

// renderDomainSection lists the domain identifiers the wave touches.
func renderDomainSection(in RivalContractInput) string {
	var b strings.Builder
	if cluster := strings.TrimSpace(in.ClusterName); cluster != "" {
		fmt.Fprintf(&b, "- Cluster: %s\n", cluster)
	}
	if len(in.IssueIDs) > 0 {
		ids := append([]string(nil), in.IssueIDs...)
		sort.Strings(ids)
		fmt.Fprintf(&b, "- Issues: %s\n", strings.Join(ids, ", "))
	}
	// Express the dispatch pattern in event/command grammar so consumers
	// can pattern-match on the contract.
	fmt.Fprintf(&b, "- Command: implementation tool will execute the steps listed below.\n")
	fmt.Fprintf(&b, "- Event: wave was specified by sightjack and ready for implementation.\n")
	if b.Len() == 0 {
		return "- (no explicit domain context recorded)"
	}
	return b.String()
}

// renderDecisionsSection emits the discussion summary or the canonical
// placeholder when no discussion is available.
func renderDecisionsSection(in RivalContractInput) string {
	if summary := strings.TrimSpace(in.DecisionsSummary); summary != "" {
		return summary
	}
	return "- " + decisionsPlaceholder
}

// renderStepsSection renders ordered implementation steps. Each step
// includes a relative target hint when one is implied, and an acceptance
// signal derived from the action description.
func renderStepsSection(in RivalContractInput) string {
	if len(in.ImplementationActs) == 0 {
		return "- (no implementation steps)"
	}
	var b strings.Builder
	for i, a := range in.ImplementationActs {
		fmt.Fprintf(&b, "%d. %s\n", i+1, fallback(a.Description, "Implementation step"))
		if id := strings.TrimSpace(a.IssueID); id != "" {
			fmt.Fprintf(&b, "   - Issue: %s\n", id)
		}
		if detail := strings.TrimSpace(a.Detail); detail != "" {
			fmt.Fprintf(&b, "   - Detail: %s\n", detail)
		}
		if t := strings.TrimSpace(a.Type); t != "" {
			fmt.Fprintf(&b, "   - Action type: %s\n", t)
		}
	}
	return b.String()
}

// renderBoundariesSection lists what the implementer must NOT do, the
// issue-management actions sightjack already handled, and any additional
// boundary lines passed in by the caller.
func renderBoundariesSection(in RivalContractInput) string {
	var b strings.Builder
	for _, a := range in.IssueMgmtActs {
		desc := fallback(a.Description, "(no description)")
		t := strings.TrimSpace(a.Type)
		if t == "" {
			t = "issue-management"
		}
		fmt.Fprintf(&b, "- Already applied by sightjack [%s]: %s\n", t, desc)
	}
	for _, line := range in.AdditionalBoundaries {
		s := strings.TrimSpace(line)
		if s == "" {
			continue
		}
		fmt.Fprintf(&b, "- %s\n", s)
	}
	fmt.Fprintf(&b, "- Do not modify issue-management state already applied above.\n")
	fmt.Fprintf(&b, "- Do not expand scope beyond the steps listed in this contract.\n")
	return b.String()
}

// renderEvidenceSection emits supported machine-readable evidence bullets
// (test/lint/check/semgrep + nfr.* when applicable) plus optional caller-
// supplied prose. The renderer is conservative: it always writes at least
// one supported grammar bullet so consumers can rely on the section being
// non-empty.
func renderEvidenceSection(in RivalContractInput) string {
	var b strings.Builder
	// Canonical evidence expectations any tool can satisfy with its
	// standard quality-gate recipes. These are advisory: the contract does
	// not authorize arbitrary command execution.
	fmt.Fprintf(&b, "- test: just test\n")
	fmt.Fprintf(&b, "- lint: just lint\n")
	for _, a := range in.ImplementationActs {
		desc := strings.TrimSpace(a.Description)
		if desc == "" {
			continue
		}
		fmt.Fprintf(&b, "- Acceptance: %s\n", desc)
	}
	for _, line := range in.AdditionalEvidence {
		s := strings.TrimSpace(line)
		if s == "" {
			continue
		}
		fmt.Fprintf(&b, "- %s\n", s)
	}
	return b.String()
}

// RenderRivalContractMetadata renders the OPTIONAL Rival Contract v1.1
// metadata key-value lines that should travel with the rendered body via
// the D-Mail YAML frontmatter. The function emits ONLY keys whose producer
// has explicitly populated a non-empty value; legacy v1 inputs (no
// DomainStyle) yield an empty string so the rendered metadata block is
// bit-identical to v1.
//
// The output format mirrors the wire-level YAML representation
// (`<key>: <value>` per line) so callers can append it directly to an
// existing metadata map or assert on it in tests without first parsing
// YAML. The renderer never invents values.
func RenderRivalContractMetadata(in RivalContractInput) string {
	var b strings.Builder
	if style := strings.TrimSpace(in.DomainStyle); style != "" {
		fmt.Fprintf(&b, "domain_style: %s\n", style)
	}
	return b.String()
}

// titleOrDefault returns the rendered title or a short fallback.
func titleOrDefault(in RivalContractInput) string {
	if t := strings.TrimSpace(in.Title); t != "" {
		return t
	}
	return "Untitled Wave"
}

// fallback returns s if non-empty (after trimming), otherwise def.
func fallback(s, def string) string {
	if t := strings.TrimSpace(s); t != "" {
		return t
	}
	return def
}
