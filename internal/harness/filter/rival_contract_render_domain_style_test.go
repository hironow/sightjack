package filter_test

import (
	"strings"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/harness/filter"
)

// Phase 1.1A — Renderer DomainStyle tests (Rival Contract v1.1).
//
// Plan: refs/plans/2026-05-03-rival-contract-v1-1-extensions.md §"Phase 1.1A"
//
// The renderer MUST emit a `domain_style: <value>` line in the rendered
// metadata block ONLY when the producer caller has explicitly populated
// RivalContractInput.DomainStyle. An empty DomainStyle MUST yield bit-
// identical output to the v1 renderer (no domain_style line emitted).

// renderedMetadataBlock returns the Markdown metadata block produced by the
// renderer's metadata-emission helper, isolated from the contract body. The
// renderer surfaces the metadata block via a dedicated helper because the
// metadata travels via the D-Mail YAML frontmatter, not the body — this
// helper materialises the same key-value lines so the test can assert on
// them directly without parsing the YAML wire format.
func renderedMetadataBlock(t *testing.T, in filter.RivalContractInput) string {
	t.Helper()
	return filter.RenderRivalContractMetadata(in)
}

func TestRenderRivalContract_DomainStyleEmitted_WhenNonEmpty(t *testing.T) {
	// given input with explicit DomainStyle set by the producer
	in := filter.RivalContractInput{
		Title:       "Add session expiry enforcement",
		ClusterName: "auth",
		IssueIDs:    []string{"MY-1"},
		ImplementationActs: []domain.WaveAction{
			{Type: "implement", IssueID: "MY-1", Description: "Implement"},
		},
		DomainStyle: "event-sourced",
	}

	// when
	meta := renderedMetadataBlock(t, in)

	// then
	if !strings.Contains(meta, "domain_style: event-sourced") {
		t.Errorf("expected metadata block to contain `domain_style: event-sourced`, got: %q", meta)
	}
}

func TestRenderRivalContract_DomainStyleOmitted_LegacyOutputBitIdentical(t *testing.T) {
	// given two inputs identical except for DomainStyle ("" vs unset)
	base := filter.RivalContractInput{
		Title:       "Add session expiry enforcement",
		ClusterName: "auth",
		IssueIDs:    []string{"MY-1"},
		ImplementationActs: []domain.WaveAction{
			{Type: "implement", IssueID: "MY-1", Description: "Implement"},
		},
	}
	withEmpty := base
	withEmpty.DomainStyle = ""

	// when
	metaUnset := renderedMetadataBlock(t, base)
	metaEmpty := renderedMetadataBlock(t, withEmpty)

	// then: empty == unset, bit-identical
	if metaUnset != metaEmpty {
		t.Errorf("empty DomainStyle must produce bit-identical metadata to unset, diff:\n unset=%q\n empty=%q", metaUnset, metaEmpty)
	}
	// then: no domain_style line emitted in the legacy path
	if strings.Contains(metaUnset, "domain_style") {
		t.Errorf("legacy/empty DomainStyle must NOT emit a domain_style line, got: %q", metaUnset)
	}
}
