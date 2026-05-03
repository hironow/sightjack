package filter_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/sightjack/internal/harness/filter"
)

func readFixture(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("testdata", "rival", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(data)
}

func TestParseRivalContractBody_ValidV1(t *testing.T) {
	// given
	body := readFixture(t, "valid-v1.md")

	// when
	contract, ok, err := filter.ParseRivalContractBody(body)

	// then
	if err != nil {
		t.Fatalf("ParseRivalContractBody: unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("ParseRivalContractBody: expected ok=true for valid v1 body")
	}
	if contract.Title != "Add session expiry enforcement" {
		t.Errorf("Title: got %q", contract.Title)
	}
	if !strings.Contains(contract.Intent, "Prevent expired sessions") {
		t.Errorf("Intent missing expected text: %q", contract.Intent)
	}
	if !strings.Contains(contract.Domain, "validate session for request") {
		t.Errorf("Domain missing expected text: %q", contract.Domain)
	}
	if !strings.Contains(contract.Decisions, "Enforce expiry in middleware") {
		t.Errorf("Decisions missing expected text: %q", contract.Decisions)
	}
	if !strings.Contains(contract.Steps, "Add expiry check to auth middleware") {
		t.Errorf("Steps missing expected text: %q", contract.Steps)
	}
	if !strings.Contains(contract.Boundaries, "Do not add OAuth") {
		t.Errorf("Boundaries missing expected text: %q", contract.Boundaries)
	}
	if !strings.Contains(contract.Evidence, "test: just test") {
		t.Errorf("Evidence missing expected text: %q", contract.Evidence)
	}
}

func TestParseRivalContractBody_LegacyReturnsFalse(t *testing.T) {
	// given
	body := readFixture(t, "legacy-spec.md")

	// when
	_, ok, err := filter.ParseRivalContractBody(body)

	// then
	if err != nil {
		t.Fatalf("ParseRivalContractBody on legacy body must not error: %v", err)
	}
	if ok {
		t.Fatal("ParseRivalContractBody: expected ok=false for legacy body without # Contract: heading")
	}
}

func TestParseRivalContractBody_PartialReturnsError(t *testing.T) {
	// given
	body := readFixture(t, "partial-v1.md")

	// when
	_, ok, err := filter.ParseRivalContractBody(body)

	// then
	if err == nil {
		t.Fatal("ParseRivalContractBody: expected error for partial v1 body")
	}
	if ok {
		t.Errorf("ParseRivalContractBody: expected ok=false on error, got ok=true")
	}
}

func TestParseEvidenceItems_ParsesSupportedKeys(t *testing.T) {
	// given
	evidence := strings.Join([]string{
		"- check: just check",
		"- test: just test",
		"- lint: just lint",
		"- semgrep: just semgrep",
		"- nfr.p95_latency_ms: <= 200",
		"- nfr.error_rate_percent: <= 1",
		"- nfr.success_rate_percent: >= 99",
		"- nfr.target_rps: >= 50",
	}, "\n")

	// when
	items := filter.ParseEvidenceItems(evidence)

	// then
	want := map[string]struct {
		Operator string
		Value    string
	}{
		"check":                    {"", "just check"},
		"test":                     {"", "just test"},
		"lint":                     {"", "just lint"},
		"semgrep":                  {"", "just semgrep"},
		"nfr.p95_latency_ms":       {"<=", "200"},
		"nfr.error_rate_percent":   {"<=", "1"},
		"nfr.success_rate_percent": {">=", "99"},
		"nfr.target_rps":           {">=", "50"},
	}
	if len(items) != len(want) {
		t.Fatalf("ParseEvidenceItems: got %d items, want %d (items=%+v)", len(items), len(want), items)
	}
	for _, item := range items {
		expected, found := want[item.Key]
		if !found {
			t.Errorf("unexpected key %q", item.Key)
			continue
		}
		if item.Operator != expected.Operator {
			t.Errorf("key %q: operator got %q want %q", item.Key, item.Operator, expected.Operator)
		}
		if item.Value != expected.Value {
			t.Errorf("key %q: value got %q want %q", item.Key, item.Value, expected.Value)
		}
	}
}

func TestParseEvidenceItems_IgnoresUnknownAndProse(t *testing.T) {
	// given
	evidence := strings.Join([]string{
		"- Add a regression test for expired sessions.",
		"- test: just test",
		"- unknown.key: 1",
		"- nfr.unknown_metric: <= 99",
		"Plain prose without bullet.",
		"- still prose without colon",
	}, "\n")

	// when
	items := filter.ParseEvidenceItems(evidence)

	// then
	if len(items) != 1 {
		t.Fatalf("ParseEvidenceItems: expected 1 item (only test), got %d (items=%+v)", len(items), items)
	}
	if items[0].Key != "test" {
		t.Errorf("expected only key 'test', got %q", items[0].Key)
	}
	if items[0].Value != "just test" {
		t.Errorf("expected value 'just test', got %q", items[0].Value)
	}
}

func TestDeriveContractID_PrefersWaveID(t *testing.T) {
	// when
	id, err := filter.DeriveContractID("auth-session-expiry", []string{"ISS-2", "ISS-1"}, "auth-cluster")

	// then
	if err != nil {
		t.Fatalf("DeriveContractID: unexpected error: %v", err)
	}
	if id != "auth-session-expiry" {
		t.Errorf("DeriveContractID: expected wave ID, got %q", id)
	}
}

func TestDeriveContractID_FallsBackDeterministically(t *testing.T) {
	// when no wave ID is present, issue IDs are used in sorted order
	id, err := filter.DeriveContractID("", []string{"ISS-2", "ISS-1"}, "auth-cluster")

	// then
	if err != nil {
		t.Fatalf("DeriveContractID: unexpected error: %v", err)
	}
	if id != "ISS-1+ISS-2" {
		t.Errorf("DeriveContractID: expected sorted issue ID join, got %q", id)
	}

	// when neither wave nor issue IDs are present, cluster name is used
	id2, err := filter.DeriveContractID("", nil, "auth-cluster")
	if err != nil {
		t.Fatalf("DeriveContractID cluster fallback: %v", err)
	}
	if id2 != "auth-cluster" {
		t.Errorf("DeriveContractID cluster: got %q", id2)
	}
}

func TestDeriveContractID_RejectsDMailNameFallback(t *testing.T) {
	// when no wave / issues / cluster is available
	id, err := filter.DeriveContractID("", nil, "")

	// then
	if err == nil {
		t.Fatalf("DeriveContractID: expected error when no stable input, got id=%q", id)
	}
	if !errors.Is(err, filter.ErrContractIDUnavailable) {
		t.Errorf("DeriveContractID: expected ErrContractIDUnavailable, got %v", err)
	}
}

func TestParseRivalContractMetadata_ValidV1(t *testing.T) {
	// given
	meta := map[string]string{
		"contract_schema":   "rival-contract-v1",
		"contract_id":       "auth-session-expiry",
		"contract_revision": "2",
		"supersedes":        "spec-auth-session-expiry_a3f2b7c4",
	}

	// when
	parsed, ok, err := filter.ParseRivalContractMetadata(meta)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true for valid metadata")
	}
	if parsed.Schema != "rival-contract-v1" {
		t.Errorf("Schema: got %q", parsed.Schema)
	}
	if parsed.ID != "auth-session-expiry" {
		t.Errorf("ID: got %q", parsed.ID)
	}
	if parsed.Revision != 2 {
		t.Errorf("Revision: got %d", parsed.Revision)
	}
	if parsed.Supersedes != "spec-auth-session-expiry_a3f2b7c4" {
		t.Errorf("Supersedes: got %q", parsed.Supersedes)
	}
}

func TestParseRivalContractMetadata_LegacyReturnsFalse(t *testing.T) {
	// given metadata with no contract_schema
	meta := map[string]string{"foo": "bar"}

	// when
	_, ok, err := filter.ParseRivalContractMetadata(meta)

	// then
	if err != nil {
		t.Fatalf("legacy metadata must not error, got %v", err)
	}
	if ok {
		t.Fatal("expected ok=false for legacy metadata without contract_schema")
	}
}

func TestParseRivalContractMetadata_RejectsDMailNameContractID(t *testing.T) {
	// given metadata where contract_id resembles a D-Mail name
	meta := map[string]string{
		"contract_schema":   "rival-contract-v1",
		"contract_id":       "spec-auth-session-expiry_a3f2b7c4",
		"contract_revision": "1",
	}

	// when
	_, _, err := filter.ParseRivalContractMetadata(meta)

	// then
	if err == nil {
		t.Fatal("expected error when contract_id matches D-Mail name pattern")
	}
}
