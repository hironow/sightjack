// Package usecase rival_export_test.go: Phase 1.1B golden tests for the
// Rival Contract v1 → OpenSPDD REASONS Canvas projection.
//
// Plan: refs/plans/2026-05-03-rival-contract-v1-1-extensions.md §"Phase 1.1B"
//
// The projection MUST be deterministic, pure, and bit-for-bit reproducible
// given the same inputs. These tests pin the markdown shape via golden
// fixture files under internal/harness/filter/testdata/rival/expected/.
package usecase_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/sightjack/internal/harness"
	"github.com/hironow/sightjack/internal/usecase"
)

// readFixture loads a golden or input fixture under sj's harness testdata.
func readFixture(t *testing.T, rel string) string {
	t.Helper()
	path := filepath.Join("..", "harness", "filter", "testdata", "rival", rel)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", rel, err)
	}
	return string(data)
}

// parseFixtureContract parses a fixture body into a RivalContract. Tests own
// metadata construction so they can vary contract_revision/supersedes/
// domain_style independently.
func parseFixtureContract(t *testing.T, fixture string) harness.RivalContract {
	t.Helper()
	body := readFixture(t, fixture)
	contract, ok, err := harness.ParseRivalContractBody(body)
	if err != nil {
		t.Fatalf("parse fixture %s: %v", fixture, err)
	}
	if !ok {
		t.Fatalf("fixture %s did not parse as Rival Contract", fixture)
	}
	return contract
}

func TestExportToReasonsCanvas_MapsAllSections(t *testing.T) {
	// given a v1 contract and v1.1 metadata.
	contract := parseFixtureContract(t, "valid-v1.md")
	meta := harness.RivalContractMetadata{
		Schema:     harness.SchemaRivalContractV1,
		ID:         "wave-auth-expiry",
		Revision:   1,
		Supersedes: "",
	}

	// when the projection runs.
	got, err := usecase.ExportToReasonsCanvas(contract, meta, "spec-auth_aaaaaaaa")
	if err != nil {
		t.Fatalf("ExportToReasonsCanvas: %v", err)
	}

	// then every REASONS Canvas section must be present at least once.
	wantHeadings := []string{
		"# Add session expiry enforcement",
		"## Requirements",
		"## Entities",
		"## Approach",
		"## Structure",
		"## Operations",
		"## Norms",
		"## Safeguards",
		"## Validation",
		"## Sync",
	}
	for _, h := range wantHeadings {
		if !strings.Contains(got, h) {
			t.Errorf("output missing heading %q\nfull output:\n%s", h, got)
		}
	}
}

func TestExportToReasonsCanvas_NormsVsSafeguardsSplit_DeterministicHeuristic(t *testing.T) {
	// given a contract with explicit Norm and Safeguard boundary bullets.
	contract := harness.RivalContract{
		Title:      "Heuristic split",
		Intent:     "- Verify split rule applies deterministically.",
		Domain:     "- Auth subsystem.",
		Decisions:  "- Apply rule based on bullet prefix.",
		Steps:      "1. Implement check.\n   - Target: `internal/auth.go`",
		Boundaries: "- Do not add OAuth.\n- Don't introduce caching.\n- Never bypass middleware.\n- Forbidden: schema migrations.\n- Use existing repository for persistence.\n- Preserve current error responses.",
		Evidence:   "- test: just test",
	}
	meta := harness.RivalContractMetadata{
		Schema:   harness.SchemaRivalContractV1,
		ID:       "split-rule",
		Revision: 1,
	}

	// when.
	got, err := usecase.ExportToReasonsCanvas(contract, meta, "spec-split_aaaaaaaa")
	if err != nil {
		t.Fatalf("ExportToReasonsCanvas: %v", err)
	}

	// then "Do not foo", "Don't bar", "Never baz", "Forbidden quux" land in Safeguards.
	safeguards := sectionBetween(t, got, "## Safeguards", "## Validation")
	for _, s := range []string{"Do not add OAuth", "Don't introduce caching", "Never bypass middleware", "Forbidden: schema migrations"} {
		if !strings.Contains(safeguards, s) {
			t.Errorf("safeguards missing %q\n--- safeguards ---\n%s", s, safeguards)
		}
	}

	// and "Use existing", "Preserve current" land in Norms.
	norms := sectionBetween(t, got, "## Norms", "## Safeguards")
	for _, n := range []string{"Use existing repository for persistence", "Preserve current error responses"} {
		if !strings.Contains(norms, n) {
			t.Errorf("norms missing %q\n--- norms ---\n%s", n, norms)
		}
	}

	// and the Safeguards bullets must NOT appear in Norms.
	for _, s := range []string{"Do not add OAuth", "Don't introduce caching", "Never bypass middleware", "Forbidden:"} {
		if strings.Contains(norms, s) {
			t.Errorf("norms unexpectedly contains safeguard bullet %q\n--- norms ---\n%s", s, norms)
		}
	}
}

func TestExportToReasonsCanvas_AddsSyncSectionFromMetadata(t *testing.T) {
	// given a contract revision 3 superseding a previous D-Mail name.
	contract := parseFixtureContract(t, "valid-v1.md")
	meta := harness.RivalContractMetadata{
		Schema:     harness.SchemaRivalContractV1,
		ID:         "wave-auth-expiry",
		Revision:   3,
		Supersedes: "spec-auth_bbbbbbbb",
	}

	// when.
	got, err := usecase.ExportToReasonsCanvas(contract, meta, "spec-auth_cccccccc")
	if err != nil {
		t.Fatalf("ExportToReasonsCanvas: %v", err)
	}

	// then the Sync section embeds source name, revision, and supersedes.
	want := "Source: D-Mail spec-auth_cccccccc, revision 3, supersedes spec-auth_bbbbbbbb"
	if !strings.Contains(got, want) {
		t.Errorf("output missing Sync line %q\n%s", want, got)
	}
}

func TestExportToReasonsCanvas_AddsSyncSectionWithNoneWhenSupersedesEmpty(t *testing.T) {
	contract := parseFixtureContract(t, "valid-v1.md")
	meta := harness.RivalContractMetadata{
		Schema:     harness.SchemaRivalContractV1,
		ID:         "wave-auth-expiry",
		Revision:   1,
		Supersedes: "",
	}

	got, err := usecase.ExportToReasonsCanvas(contract, meta, "spec-auth_aaaaaaaa")
	if err != nil {
		t.Fatalf("ExportToReasonsCanvas: %v", err)
	}

	want := "Source: D-Mail spec-auth_aaaaaaaa, revision 1, supersedes none"
	if !strings.Contains(got, want) {
		t.Errorf("output missing 'supersedes none' Sync line\n%s", got)
	}
}

func TestExportToReasonsCanvas_EventSourcedDomainExtractsCommandEventReadModel(t *testing.T) {
	// given a contract whose body uses event-sourcing vocabulary AND v1.1
	// metadata explicitly says domain_style=event-sourced.
	contract := parseFixtureContract(t, "event-sourced-v1.md")
	meta := harness.RivalContractMetadata{
		Schema:      harness.SchemaRivalContractV1,
		ID:          "wave-event-sourced",
		Revision:    1,
		DomainStyle: harness.DomainStyleEventSourced,
	}

	// when.
	got, err := usecase.ExportToReasonsCanvas(contract, meta, "spec-es_aaaaaaaa")
	if err != nil {
		t.Fatalf("ExportToReasonsCanvas: %v", err)
	}

	// then Entities section contains Commands / Events / Read Models /
	// Aggregates as sub-headings (### or labelled bullets) with the
	// extracted values.
	entities := sectionBetween(t, got, "## Entities", "## Approach")
	wantBuckets := []struct {
		label string
		value string
	}{
		{"Commands", "ValidateSession"},
		{"Events", "SessionValidationFailed"},
		{"Read Models", "AuthMiddlewareView"},
		{"Aggregates", "Session"},
	}
	for _, w := range wantBuckets {
		if !strings.Contains(entities, w.label) {
			t.Errorf("entities missing bucket label %q\n--- entities ---\n%s", w.label, entities)
		}
		if !strings.Contains(entities, w.value) {
			t.Errorf("entities missing extracted value %q for bucket %q\n--- entities ---\n%s", w.value, w.label, entities)
		}
	}
}

func TestExportToReasonsCanvas_GenericDomainPassesThroughVerbatim(t *testing.T) {
	// given a generic-style contract: even though valid-v1.md mentions
	// "Command:" prose, with DomainStyle empty the projection MUST NOT
	// extract event-sourcing buckets.
	contract := parseFixtureContract(t, "valid-v1.md")
	meta := harness.RivalContractMetadata{
		Schema:   harness.SchemaRivalContractV1,
		ID:       "wave-auth-expiry",
		Revision: 1,
	}

	// when.
	got, err := usecase.ExportToReasonsCanvas(contract, meta, "spec-auth_aaaaaaaa")
	if err != nil {
		t.Fatalf("ExportToReasonsCanvas: %v", err)
	}

	entities := sectionBetween(t, got, "## Entities", "## Approach")
	for _, label := range []string{"### Commands", "### Events", "### Read Models", "### Aggregates"} {
		if strings.Contains(entities, label) {
			t.Errorf("generic style must not emit event-sourced sub-heading %q\n--- entities ---\n%s", label, entities)
		}
	}
	// Domain bullet content must still be passed through verbatim.
	if !strings.Contains(entities, "validate session for request") {
		t.Errorf("entities missing verbatim domain content\n%s", entities)
	}
}

func TestExportToReasonsCanvas_LegacyV1NoMetadataReturnsError(t *testing.T) {
	// given a parsed contract but a metadata struct with empty Schema (i.e.
	// Rival Contract v1 metadata not present — pre-v1 raw spec).
	contract := parseFixtureContract(t, "valid-v1.md")
	meta := harness.RivalContractMetadata{
		Schema:   "",
		ID:       "",
		Revision: 0,
	}

	// when.
	_, err := usecase.ExportToReasonsCanvas(contract, meta, "spec-auth_aaaaaaaa")

	// then the projection refuses to export (schema is required).
	if err == nil {
		t.Fatal("expected error when contract metadata schema is empty, got nil")
	}
	if !strings.Contains(err.Error(), "rival-contract-v1") {
		t.Errorf("error should mention required schema; got: %v", err)
	}
}

func TestExportToReasonsCanvas_GoldenValidV1(t *testing.T) {
	// given v1 contract + minimal metadata.
	contract := parseFixtureContract(t, "valid-v1.md")
	meta := harness.RivalContractMetadata{
		Schema:     harness.SchemaRivalContractV1,
		ID:         "wave-auth-expiry",
		Revision:   1,
		Supersedes: "",
	}

	// when.
	got, err := usecase.ExportToReasonsCanvas(contract, meta, "spec-auth_aaaaaaaa")
	if err != nil {
		t.Fatalf("ExportToReasonsCanvas: %v", err)
	}

	// then output bit-matches the golden file.
	want := readFixture(t, "expected/reasons-canvas.md")
	if got != want {
		t.Errorf("golden mismatch (valid-v1)\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestExportToReasonsCanvas_GoldenEventSourcedV1(t *testing.T) {
	contract := parseFixtureContract(t, "event-sourced-v1.md")
	meta := harness.RivalContractMetadata{
		Schema:      harness.SchemaRivalContractV1,
		ID:          "wave-event-sourced",
		Revision:    2,
		Supersedes:  "spec-es_aaaaaaaa",
		DomainStyle: harness.DomainStyleEventSourced,
	}

	got, err := usecase.ExportToReasonsCanvas(contract, meta, "spec-es_bbbbbbbb")
	if err != nil {
		t.Fatalf("ExportToReasonsCanvas: %v", err)
	}

	want := readFixture(t, "expected/reasons-canvas-event-sourced.md")
	if got != want {
		t.Errorf("golden mismatch (event-sourced-v1)\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// sectionBetween returns the substring of s starting after the line equal to
// startHeading up to the line equal to endHeading (exclusive). It is used in
// assertions that need to scope expectations to a single REASONS Canvas
// section.
func sectionBetween(t *testing.T, s, startHeading, endHeading string) string {
	t.Helper()
	startIdx := strings.Index(s, startHeading)
	if startIdx < 0 {
		t.Fatalf("section %q not found in output:\n%s", startHeading, s)
	}
	rest := s[startIdx+len(startHeading):]
	endIdx := strings.Index(rest, endHeading)
	if endIdx < 0 {
		// final section — return rest of buffer.
		return rest
	}
	return rest[:endIdx]
}
