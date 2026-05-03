package filter_test

import (
	"strings"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/harness/filter"
)

// rivalContractHeadingsInOrder lists the canonical Rival Contract v1
// section headings in the exact order they must appear in the body.
var rivalContractHeadingsInOrder = []string{
	"## Intent",
	"## Domain",
	"## Decisions",
	"## Steps",
	"## Boundaries",
	"## Evidence",
}

func TestRenderRivalContract_IncludesAllHeadingsInOrder(t *testing.T) {
	// given
	in := filter.RivalContractInput{
		Title:       "Add session expiry enforcement",
		Description: "Prevent expired sessions from authorizing API calls.",
		ClusterName: "auth",
		IssueIDs:    []string{"MY-1", "MY-2"},
		ImplementationActs: []domain.WaveAction{
			{Type: "implement", IssueID: "MY-1", Description: "Add expiry check", Detail: "Edit auth middleware"},
			{Type: "verify", IssueID: "MY-2", Description: "Add unit tests"},
		},
		IssueMgmtActs: []domain.WaveAction{
			{Type: "add_dod", IssueID: "MY-1", Description: "Add DoD"},
		},
	}

	// when
	body := filter.RenderRivalContract(in)

	// then: title heading
	if !strings.HasPrefix(strings.TrimLeft(body, "\n"), "# Contract: Add session expiry enforcement") {
		t.Errorf("body must start with `# Contract: <title>` heading, got first 80 chars: %q", firstN(body, 80))
	}
	// then: section headings appear in canonical order
	prev := -1
	for _, h := range rivalContractHeadingsInOrder {
		idx := strings.Index(body, "\n"+h+"\n")
		if idx < 0 {
			// allow heading at very start (no leading newline) too
			if strings.HasPrefix(body, h+"\n") {
				idx = 0
			} else {
				t.Errorf("missing heading %q in body", h)
				continue
			}
		}
		if idx <= prev {
			t.Errorf("heading %q appeared at byte offset %d but previous heading was at %d (out of order)", h, idx, prev)
		}
		prev = idx
	}
}

func TestRenderRivalContract_RoundTripsThroughParser(t *testing.T) {
	// given: a fully populated input
	in := filter.RivalContractInput{
		Title:       "Add expiry check",
		Description: "Reject expired sessions at middleware.",
		ClusterName: "auth",
		IssueIDs:    []string{"MY-10"},
		ImplementationActs: []domain.WaveAction{
			{Type: "implement", IssueID: "MY-10", Description: "Add middleware check", Detail: "Validate expiry"},
		},
		IssueMgmtActs: []domain.WaveAction{
			{Type: "add_dod", IssueID: "MY-10", Description: "DoD update"},
		},
	}

	// when
	body := filter.RenderRivalContract(in)

	// then: the rendered body parses cleanly via the canonical parser.
	contract, ok, err := filter.ParseRivalContractBody(body)
	if err != nil {
		t.Fatalf("ParseRivalContractBody: %v", err)
	}
	if !ok {
		t.Fatal("ParseRivalContractBody: ok=false; renderer output must satisfy parser")
	}
	if contract.Title != "Add expiry check" {
		t.Errorf("Title roundtrip: got %q, want %q", contract.Title, "Add expiry check")
	}
	if contract.Intent == "" {
		t.Error("Intent must not be empty")
	}
	if contract.Domain == "" {
		t.Error("Domain must not be empty")
	}
	if contract.Decisions == "" {
		t.Error("Decisions must not be empty (must default to placeholder text)")
	}
	if contract.Steps == "" {
		t.Error("Steps must not be empty")
	}
	if contract.Boundaries == "" {
		t.Error("Boundaries must not be empty")
	}
	if contract.Evidence == "" {
		t.Error("Evidence must not be empty (must include at least DoD/test expectation)")
	}
}

func TestRenderRivalContract_StepsContainImplementationActsOnly(t *testing.T) {
	// given: mixed acts
	in := filter.RivalContractInput{
		Title: "Mixed wave",
		ImplementationActs: []domain.WaveAction{
			{Type: "implement", IssueID: "MY-1", Description: "Implement feature"},
		},
		IssueMgmtActs: []domain.WaveAction{
			{Type: "add_dod", IssueID: "MY-2", Description: "DoD action"},
		},
	}

	// when
	body := filter.RenderRivalContract(in)
	contract, ok, err := filter.ParseRivalContractBody(body)
	if err != nil || !ok {
		t.Fatalf("parser must accept rendered body: ok=%v err=%v", ok, err)
	}

	// then
	if !strings.Contains(contract.Steps, "Implement feature") {
		t.Errorf("Steps must contain implementation action, got: %q", contract.Steps)
	}
	if strings.Contains(contract.Steps, "DoD action") {
		t.Errorf("Steps must NOT contain issue-management action, got: %q", contract.Steps)
	}
	if !strings.Contains(contract.Boundaries, "DoD action") &&
		!strings.Contains(contract.Boundaries, "add_dod") {
		t.Errorf("Boundaries must reference filtered issue-management action, got: %q", contract.Boundaries)
	}
}

func TestRenderRivalContract_DecisionsHasPlaceholderWhenAbsent(t *testing.T) {
	// given: input with no DecisionsSummary populated
	in := filter.RivalContractInput{
		Title: "No design discussion",
		ImplementationActs: []domain.WaveAction{
			{Type: "implement", IssueID: "MY-1", Description: "Action"},
		},
	}

	// when
	body := filter.RenderRivalContract(in)
	contract, ok, _ := filter.ParseRivalContractBody(body)
	if !ok {
		t.Fatal("parser rejected rendered body")
	}

	// then: per plan, an empty discussion must render an explicit placeholder.
	if !strings.Contains(contract.Decisions, "No explicit design decision recorded") {
		t.Errorf("Decisions must contain placeholder text when discussion is empty, got: %q", contract.Decisions)
	}
}

func TestRenderRivalContract_EvidenceUsesSupportedGrammar(t *testing.T) {
	// given: input with acceptance criteria
	in := filter.RivalContractInput{
		Title: "Evidence grammar test",
		ImplementationActs: []domain.WaveAction{
			{Type: "implement", IssueID: "MY-1", Description: "Implement", Detail: "Acceptance: tests pass"},
		},
	}

	// when
	body := filter.RenderRivalContract(in)
	contract, ok, _ := filter.ParseRivalContractBody(body)
	if !ok {
		t.Fatal("parser rejected rendered body")
	}

	// then: evidence section must include at least one supported machine-readable bullet.
	items := filter.ParseEvidenceItems(contract.Evidence)
	if len(items) == 0 {
		t.Fatalf("Evidence section must include at least one supported grammar bullet (test/check/lint/semgrep/nfr.*), got: %q", contract.Evidence)
	}
	supported := map[string]struct{}{
		"check":                    {},
		"test":                     {},
		"lint":                     {},
		"semgrep":                  {},
		"nfr.p95_latency_ms":       {},
		"nfr.error_rate_percent":   {},
		"nfr.success_rate_percent": {},
		"nfr.target_rps":           {},
	}
	for _, it := range items {
		if _, ok := supported[it.Key]; !ok {
			t.Errorf("Evidence emitted unsupported key %q", it.Key)
		}
	}
}

func TestRenderRivalContract_NoAbsolutePathsInBody(t *testing.T) {
	// given: typical input
	in := filter.RivalContractInput{
		Title: "Path leak guard",
		ImplementationActs: []domain.WaveAction{
			{Type: "implement", IssueID: "MY-1", Description: "Implement", Detail: "edit handler.go"},
		},
	}

	// when
	body := filter.RenderRivalContract(in)

	// then: per plan, repository-relative paths only.
	for _, fragment := range []string{"/Users/", "/home/", "C:\\"} {
		if strings.Contains(body, fragment) {
			t.Errorf("rendered body must not contain absolute path fragment %q", fragment)
		}
	}
}

// firstN returns the first n bytes of s for diagnostic messages.
func firstN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
