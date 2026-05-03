// Package usecase rival_export_json_test.go: Phase 1.1B JSON projection
// golden tests for the Rival Contract v1 → REASONS Canvas mapping.
//
// Plan: refs/plans/2026-05-03-rival-contract-v1-1-extensions.md §"Phase 1.1B"
//
// The JSON projection is a separate output mode that consumers (e.g. the
// future OpenSPDD CLI ingest path) can parse without touching markdown.
// Output MUST be deterministic (sorted/ordered keys via struct tags) and
// bit-for-bit reproducible against the golden file.
package usecase_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/hironow/sightjack/internal/harness/filter"
	"github.com/hironow/sightjack/internal/usecase"
)

func TestExportToReasonsCanvasJSON_DeterministicShape(t *testing.T) {
	// given a v1 contract + minimal metadata.
	contract := parseFixtureContract(t, "valid-v1.md")
	meta := filter.RivalContractMetadata{
		Schema:     filter.SchemaRivalContractV1,
		ID:         "wave-auth-expiry",
		Revision:   1,
		Supersedes: "",
	}

	// when.
	got, err := usecase.ExportToReasonsCanvasJSON(contract, meta, "spec-auth_aaaaaaaa")
	if err != nil {
		t.Fatalf("ExportToReasonsCanvasJSON: %v", err)
	}

	// then output bit-matches the JSON golden file.
	want := readFixture(t, "expected/reasons-canvas.json")
	if got != want {
		t.Errorf("JSON golden mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}

	// and it parses as JSON with the expected top-level keys.
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("JSON not parseable: %v\n%s", err, got)
	}
	for _, key := range []string{
		"title", "requirements", "entities", "approach", "structure",
		"operations", "norms", "safeguards", "validation", "sync",
	} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("expected top-level key %q in JSON output\n%s", key, got)
		}
	}
}

func TestExportToReasonsCanvasJSON_RejectsMissingSchema(t *testing.T) {
	contract := parseFixtureContract(t, "valid-v1.md")
	meta := filter.RivalContractMetadata{}

	_, err := usecase.ExportToReasonsCanvasJSON(contract, meta, "spec-auth_aaaaaaaa")
	if err == nil {
		t.Fatal("expected error when schema missing, got nil")
	}
	if !strings.Contains(err.Error(), "rival-contract-v1") {
		t.Errorf("error should mention required schema; got: %v", err)
	}
}
