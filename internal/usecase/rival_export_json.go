// Package usecase rival_export_json.go: deterministic JSON projection of
// the Rival Contract v1 → REASONS Canvas mapping.
//
// Phase 1.1B (plan: refs/plans/2026-05-03-rival-contract-v1-1-extensions.md).
//
// The JSON shape mirrors the markdown sections and is intended for
// machine consumers that prefer structured input over Markdown parsing.
// The same purity invariants apply as the Markdown projection: no I/O, no
// LLM, no env reads, deterministic output.
package usecase

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hironow/sightjack/internal/harness"
)

// reasonsCanvasJSON is the deterministic on-the-wire shape for JSON export.
// Field order in the struct fixes JSON key order for encoding/json since Go
// preserves declaration order for struct marshaling.
type reasonsCanvasJSON struct { // nosemgrep: structure.multiple-exported-structs-go -- not exported [permanent]
	Title        string                 `json:"title"`
	Requirements []string               `json:"requirements"`
	Entities     reasonsCanvasEntities  `json:"entities"`
	Approach     []string               `json:"approach"`
	Structure    []string               `json:"structure"`
	Operations   string                 `json:"operations"`
	Norms        []string               `json:"norms"`
	Safeguards   []string               `json:"safeguards"`
	Validation   []string               `json:"validation"`
	Sync         reasonsCanvasSyncBlock `json:"sync"`
}

// reasonsCanvasEntities preserves both the verbatim Domain text and, when
// applicable, the event-sourced extraction buckets.
type reasonsCanvasEntities struct {
	DomainStyle string   `json:"domain_style"`
	Verbatim    string   `json:"verbatim"`
	Commands    []string `json:"commands,omitempty"`
	Events      []string `json:"events,omitempty"`
	ReadModels  []string `json:"read_models,omitempty"`
	Aggregates  []string `json:"aggregates,omitempty"`
	Other       []string `json:"other,omitempty"`
}

// reasonsCanvasSyncBlock captures the deterministic Sync metadata as
// individual fields so JSON consumers don't have to parse the rendered
// "Source: D-Mail ..." string back apart.
type reasonsCanvasSyncBlock struct {
	SourceDMail string `json:"source_dmail"`
	Revision    int    `json:"revision"`
	Supersedes  string `json:"supersedes"`
	Line        string `json:"line"`
}

// ExportToReasonsCanvasJSON projects a Rival Contract v1 specification
// into a deterministic JSON document. Keys are stable, indentation is
// two spaces, and the output is reproducible bit-for-bit given the same
// inputs.
func ExportToReasonsCanvasJSON(rc harness.RivalContract, meta harness.RivalContractMetadata, sourceDMailName string) (string, error) {
	if meta.Schema != harness.SchemaRivalContractV1 {
		return "", fmt.Errorf("%w: got schema %q", ErrUnsupportedContractForExport, meta.Schema)
	}

	title := strings.TrimSpace(rc.Title)
	if title == "" {
		title = "Untitled Contract"
	}

	doc := reasonsCanvasJSON{
		Title:        title,
		Requirements: bulletsOf(rc.Intent),
		Entities:     entitiesJSON(rc.Domain, meta.DomainStyle),
		Approach:     bulletsOf(rc.Decisions),
		Structure:    structureTargets(rc.Steps),
		Operations:   strings.TrimSpace(rc.Steps),
		Validation:   bulletsOf(rc.Evidence),
	}
	norms, safeguards := splitBoundariesLists(rc.Boundaries)
	doc.Norms = norms
	doc.Safeguards = safeguards
	supersedes := strings.TrimSpace(meta.Supersedes)
	syncSupersedes := supersedes
	if syncSupersedes == "" {
		syncSupersedes = "none"
	}
	doc.Sync = reasonsCanvasSyncBlock{
		SourceDMail: strings.TrimSpace(sourceDMailName),
		Revision:    meta.Revision,
		Supersedes:  syncSupersedes,
		Line:        renderSync(meta, sourceDMailName),
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(doc); err != nil {
		return "", fmt.Errorf("rival export json: encode: %w", err)
	}
	// json.Encoder appends a trailing newline; preserve it for POSIX-friendly
	// file output.
	return buf.String(), nil
}

// bulletsOf splits a section body into clean bullet strings (one entry per
// `- ` prefixed line, trimmed). Non-bullet lines are kept as-is so prose
// is not silently lost. Returns an empty slice (not nil) when the body is
// empty so JSON output emits `[]` rather than `null`.
func bulletsOf(section string) []string {
	section = strings.TrimSpace(section)
	out := []string{}
	if section == "" {
		return out
	}
	for _, raw := range strings.Split(section, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "- ") {
			out = append(out, strings.TrimSpace(strings.TrimPrefix(line, "- ")))
			continue
		}
		out = append(out, line)
	}
	return out
}

// entitiesJSON constructs the JSON Entities block. When DomainStyle is
// event-sourced, bullet labels are extracted into typed buckets; otherwise
// only the verbatim text is populated.
func entitiesJSON(domain string, domainStyle string) reasonsCanvasEntities {
	domain = strings.TrimSpace(domain)
	out := reasonsCanvasEntities{
		DomainStyle: domainStyle,
		Verbatim:    domain,
	}
	if domainStyle != harness.DomainStyleEventSourced || domain == "" {
		return out
	}
	for _, raw := range strings.Split(domain, "\n") {
		line := strings.TrimSpace(raw)
		if !strings.HasPrefix(line, "- ") {
			if line != "" {
				out.Other = append(out.Other, line)
			}
			continue
		}
		body := strings.TrimSpace(strings.TrimPrefix(line, "- "))
		switch {
		case hasEventSourcedPrefix(body, "Command:"):
			out.Commands = append(out.Commands, stripPrefix(body, "Command:"))
		case hasEventSourcedPrefix(body, "Event:"):
			out.Events = append(out.Events, stripPrefix(body, "Event:"))
		case hasEventSourcedPrefix(body, "Read model:"):
			out.ReadModels = append(out.ReadModels, stripPrefix(body, "Read model:"))
		case hasEventSourcedPrefix(body, "Read Model:"):
			out.ReadModels = append(out.ReadModels, stripPrefix(body, "Read Model:"))
		case hasEventSourcedPrefix(body, "Aggregate:"):
			out.Aggregates = append(out.Aggregates, stripPrefix(body, "Aggregate:"))
		default:
			out.Other = append(out.Other, body)
		}
	}
	return out
}

// structureTargets returns the deduplicated, first-seen-ordered list of
// Step targets used in the Structure section. Pure mirror of the Markdown
// projection's renderStructure path.
func structureTargets(steps string) []string {
	steps = strings.TrimSpace(steps)
	out := []string{}
	if steps == "" {
		return out
	}
	seen := map[string]struct{}{}
	for _, raw := range strings.Split(steps, "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "- ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "- "))
		}
		if !strings.HasPrefix(line, "Target:") {
			continue
		}
		matches := stepTargetPattern.FindStringSubmatch(line)
		if len(matches) < 2 {
			continue
		}
		t := strings.TrimSpace(matches[1])
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	return out
}

// splitBoundariesLists returns the (norms, safeguards) bullet lists used
// by the JSON projection. Mirrors splitBoundaries but emits []string so the
// JSON consumer doesn't have to parse markdown bullets.
func splitBoundariesLists(boundaries string) ([]string, []string) {
	boundaries = strings.TrimSpace(boundaries)
	norms := []string{}
	safeguards := []string{}
	if boundaries == "" {
		return norms, safeguards
	}
	for _, raw := range strings.Split(boundaries, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "- ") {
			norms = append(norms, line)
			continue
		}
		body := strings.TrimSpace(strings.TrimPrefix(line, "- "))
		if isSafeguardBullet(body) {
			safeguards = append(safeguards, body)
		} else {
			norms = append(norms, body)
		}
	}
	return norms, safeguards
}
