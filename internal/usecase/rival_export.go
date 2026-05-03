// Package usecase rival_export.go: Rival Contract v1.1 → OpenSPDD REASONS
// Canvas projection.
//
// Phase 1.1B (plan: refs/plans/2026-05-03-rival-contract-v1-1-extensions.md).
//
// ExportToReasonsCanvas is a pure, deterministic projection. Given the same
// inputs it produces bit-for-bit identical Markdown output. It performs no
// I/O, no LLM calls, never mutates the source D-Mail, and reads no env vars.
//
// Mapping table (per plan §"Mapping"):
//
//	# Contract: <title>                           -> # <title>
//	## Intent                                     -> ## Requirements
//	## Domain                                     -> ## Entities
//	  (when DomainStyle == event-sourced and bullets parse cleanly into
//	   Command:/Event:/Read model:/Aggregate: prefixes, they are split
//	   into ### Commands / ### Events / ### Read Models / ### Aggregates
//	   sub-headings; otherwise the body is passed through verbatim)
//	## Decisions                                  -> ## Approach
//	(targets extracted from ## Steps)             -> ## Structure
//	## Steps                                      -> ## Operations
//	## Boundaries                                 -> ## Norms AND ## Safeguards
//	  (heuristic split: bullets prefixed with "Do not", "Don't", "Never",
//	   "Forbidden", "No " (with following space), or "Avoid " go to
//	   Safeguards; everything else goes to Norms)
//	## Evidence                                   -> ## Validation
//	metadata (revision, supersedes, source name)  -> ## Sync (deterministic)
//
// Constraints:
//   - PURE: no os.Open*/os.Write*/exec/network/env reads beyond what callers
//     already pre-resolved.
//   - DETERMINISTIC: stable ordering, no map iteration without sort.
//   - REPRODUCIBLE: same inputs MUST yield identical bytes across runs.
package usecase

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/hironow/sightjack/internal/harness/filter"
)

// ErrUnsupportedContractForExport is returned by ExportToReasonsCanvas when
// the supplied metadata is not a valid Rival Contract v1 specification (e.g.
// missing schema, used against a legacy pre-v1 raw spec body). Per plan
// §"Phase 1.1B" the export tool refuses such inputs in v1.1; revisit in v2.
var ErrUnsupportedContractForExport = errors.New("rival export: only rival-contract-v1 specifications can be exported to REASONS Canvas")

// ExportToReasonsCanvas projects a parsed Rival Contract v1 specification
// onto OpenSPDD REASONS Canvas markdown.
//
// sourceDMailName is the originating D-Mail's name (without `.md` suffix);
// it appears verbatim in the deterministic Sync line so external tools can
// trace back to the contract revision.
//
// Returns ErrUnsupportedContractForExport when the metadata is not Rival
// Contract v1 (Schema must equal filter.SchemaRivalContractV1).
func ExportToReasonsCanvas(rc filter.RivalContract, meta filter.RivalContractMetadata, sourceDMailName string) (string, error) {
	if meta.Schema != filter.SchemaRivalContractV1 {
		return "", fmt.Errorf("%w: got schema %q", ErrUnsupportedContractForExport, meta.Schema)
	}

	var b strings.Builder

	// Header: use the contract title verbatim.
	title := strings.TrimSpace(rc.Title)
	if title == "" {
		title = "Untitled Contract"
	}
	fmt.Fprintf(&b, "# %s\n\n", title)

	writeSection(&b, "Requirements", strings.TrimSpace(rc.Intent))
	writeSection(&b, "Entities", renderEntities(rc.Domain, meta.DomainStyle))
	writeSection(&b, "Approach", strings.TrimSpace(rc.Decisions))
	writeSection(&b, "Structure", renderStructure(rc.Steps))
	writeSection(&b, "Operations", strings.TrimSpace(rc.Steps))

	norms, safeguards := splitBoundaries(rc.Boundaries)
	writeSection(&b, "Norms", norms)
	writeSection(&b, "Safeguards", safeguards)
	writeSection(&b, "Validation", strings.TrimSpace(rc.Evidence))
	writeSection(&b, "Sync", renderSync(meta, sourceDMailName))

	return b.String(), nil
}

// writeSection emits a `## <heading>` block followed by trimmed body and
// trailing blank line. Empty bodies render as a single deterministic
// placeholder line so downstream readers see a stable shape.
func writeSection(b *strings.Builder, heading, body string) {
	fmt.Fprintf(b, "## %s\n\n", heading)
	body = strings.TrimSpace(body)
	if body == "" {
		body = "_(none)_"
	}
	b.WriteString(body)
	b.WriteString("\n\n")
}

// renderEntities projects the Domain section into the Entities section.
//
// When domainStyle == "event-sourced", bullets prefixed with one of the
// canonical event-sourcing labels ("Command:", "Event:", "Read model:",
// "Aggregate:") are extracted and grouped under deterministic sub-headings
// (### Commands / ### Events / ### Read Models / ### Aggregates). Bullets
// that do not match any label are appended verbatim under "### Other" so
// nothing is lost.
//
// When domainStyle is empty or any other value, the Domain body is passed
// through verbatim (preserving the v1 invariant: no NLP classification).
func renderEntities(domain string, domainStyle string) string {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return ""
	}
	if domainStyle != filter.DomainStyleEventSourced {
		return domain
	}

	commands := []string{}
	events := []string{}
	readModels := []string{}
	aggregates := []string{}
	others := []string{}

	for _, raw := range strings.Split(domain, "\n") {
		line := strings.TrimSpace(raw)
		if !strings.HasPrefix(line, "- ") {
			// Preserve non-bullet prose lines as Other so nothing is lost.
			if line != "" {
				others = append(others, line)
			}
			continue
		}
		body := strings.TrimSpace(strings.TrimPrefix(line, "- "))
		switch {
		case hasEventSourcedPrefix(body, "Command:"):
			commands = append(commands, stripPrefix(body, "Command:"))
		case hasEventSourcedPrefix(body, "Event:"):
			events = append(events, stripPrefix(body, "Event:"))
		case hasEventSourcedPrefix(body, "Read model:"):
			readModels = append(readModels, stripPrefix(body, "Read model:"))
		case hasEventSourcedPrefix(body, "Read Model:"):
			readModels = append(readModels, stripPrefix(body, "Read Model:"))
		case hasEventSourcedPrefix(body, "Aggregate:"):
			aggregates = append(aggregates, stripPrefix(body, "Aggregate:"))
		default:
			others = append(others, body)
		}
	}

	var b strings.Builder
	writeBucket(&b, "Commands", commands)
	writeBucket(&b, "Events", events)
	writeBucket(&b, "Read Models", readModels)
	writeBucket(&b, "Aggregates", aggregates)
	if len(others) > 0 {
		writeBucket(&b, "Other", others)
	}
	return strings.TrimRight(b.String(), "\n")
}

// hasEventSourcedPrefix reports whether body begins with a case-sensitive
// label followed by either end-of-string or whitespace.
func hasEventSourcedPrefix(body, prefix string) bool {
	if !strings.HasPrefix(body, prefix) {
		return false
	}
	rest := body[len(prefix):]
	if rest == "" {
		return true
	}
	r := rest[0]
	return r == ' ' || r == '\t'
}

// stripPrefix removes prefix from body (case-sensitive) and trims the
// remainder. The trailing period that often closes domain bullets is
// removed for cleaner extracted values.
func stripPrefix(body, prefix string) string {
	rest := strings.TrimSpace(strings.TrimPrefix(body, prefix))
	rest = strings.TrimSuffix(rest, ".")
	return strings.TrimSpace(rest)
}

// writeBucket emits a `### <label>` sub-heading and a list of bullets.
// Empty buckets are emitted with a `_(none)_` placeholder so consumers see
// the four canonical event-sourcing buckets in the same order regardless of
// which ones are populated.
func writeBucket(b *strings.Builder, label string, items []string) {
	fmt.Fprintf(b, "### %s\n\n", label)
	if len(items) == 0 {
		b.WriteString("_(none)_\n\n")
		return
	}
	for _, it := range items {
		fmt.Fprintf(b, "- %s\n", it)
	}
	b.WriteByte('\n')
}

// stepTargetPattern matches `Target: \`<path>\`` lines anywhere in the
// Steps body. The path is captured so renderStructure can list components
// without duplicating Steps prose.
var stepTargetPattern = regexp.MustCompile("`([^`]+)`")

// renderStructure derives the Structure section from Step targets. It scans
// lines that begin with "Target:" inside the Steps body, extracts the
// backtick-quoted path, and emits one bullet per unique target preserving
// first-seen order. Output is deterministic given the same Steps input.
func renderStructure(steps string) string {
	steps = strings.TrimSpace(steps)
	if steps == "" {
		return ""
	}
	seen := map[string]struct{}{}
	var ordered []string
	for _, raw := range strings.Split(steps, "\n") {
		line := strings.TrimSpace(raw)
		// Tolerate "- Target:" and "Target:" prefixes after the leading dash.
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
		target := strings.TrimSpace(matches[1])
		if target == "" {
			continue
		}
		if _, ok := seen[target]; ok {
			continue
		}
		seen[target] = struct{}{}
		ordered = append(ordered, target)
	}
	if len(ordered) == 0 {
		return ""
	}
	var b strings.Builder
	for _, t := range ordered {
		fmt.Fprintf(&b, "- `%s`\n", t)
	}
	return strings.TrimRight(b.String(), "\n")
}

// safeguardPrefixes are the lowercase prefixes that route a Boundaries
// bullet to the Safeguards section. The list is deliberately narrow and
// case-insensitive only on the prefix; the rest of the line is preserved.
var safeguardPrefixes = []string{
	"do not ",
	"don't ",
	"never ",
	"forbidden ",
	"forbidden:",
	"no ",
	"avoid ",
}

// splitBoundaries partitions the Boundaries body into (norms, safeguards)
// rendered as bulleted markdown. The split is deterministic and based on
// the lowercase prefix of each bullet. Non-bullet prose lines are appended
// to Norms (the positive-style section) so nothing is silently dropped.
func splitBoundaries(boundaries string) (string, string) {
	boundaries = strings.TrimSpace(boundaries)
	if boundaries == "" {
		return "", ""
	}
	var norms, safeguards strings.Builder
	for _, raw := range strings.Split(boundaries, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "- ") {
			// Carry prose lines into Norms verbatim (still trimmed).
			fmt.Fprintf(&norms, "- %s\n", line)
			continue
		}
		body := strings.TrimSpace(strings.TrimPrefix(line, "- "))
		if isSafeguardBullet(body) {
			fmt.Fprintf(&safeguards, "- %s\n", body)
		} else {
			fmt.Fprintf(&norms, "- %s\n", body)
		}
	}
	return strings.TrimRight(norms.String(), "\n"), strings.TrimRight(safeguards.String(), "\n")
}

// isSafeguardBullet reports whether bullet text is a Safeguard (forbidden
// edit) per the heuristic documented in splitBoundaries.
func isSafeguardBullet(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	for _, p := range safeguardPrefixes {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}
	return false
}

// renderSync emits the deterministic Sync section line. supersedes is
// rendered as "none" when empty so the line shape is invariant.
func renderSync(meta filter.RivalContractMetadata, sourceDMailName string) string {
	supersedes := strings.TrimSpace(meta.Supersedes)
	if supersedes == "" {
		supersedes = "none"
	}
	return fmt.Sprintf("Source: D-Mail %s, revision %d, supersedes %s",
		strings.TrimSpace(sourceDMailName), meta.Revision, supersedes)
}
