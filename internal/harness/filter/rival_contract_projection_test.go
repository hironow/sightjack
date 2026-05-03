package filter_test

import (
	"strings"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/harness/filter"
)

// Phase 1.1A — ProjectCurrentContracts copy-sync tests (Rival Contract v1.1).
//
// Plan: refs/plans/2026-05-03-rival-contract-v1-1-extensions.md §"Phase 1.1A"
//
// sj's ProjectCurrentContracts is a byte-identical copy of amadeus's canonical
// implementation (aside from package name and DMail import path). These tests
// mirror the amadeus regression suite so any drift between the two copies is
// caught locally.

// rivalDMail builds a specification D-Mail with canonical Rival Contract v1
// metadata. Mirrors amadeus's helper so the copy-sync regression is direct.
func rivalDMail(name, contractID string, revision int, supersedes string, body string) domain.DMail {
	meta := map[string]string{
		"contract_schema":   filter.SchemaRivalContractV1,
		"contract_id":       contractID,
		"contract_revision": itoa(revision),
	}
	if supersedes != "" {
		meta["supersedes"] = supersedes
	}
	return domain.DMail{
		Name:     name,
		Kind:     domain.KindSpecification,
		Body:     body,
		Metadata: meta,
	}
}

// rivalDMailWithKey is rivalDMail plus an idempotency_key so duplicate-
// delivery scenarios can be expressed unambiguously.
func rivalDMailWithKey(name, contractID string, revision int, supersedes, body, idempotencyKey string) domain.DMail {
	d := rivalDMail(name, contractID, revision, supersedes, body)
	d.Metadata["idempotency_key"] = idempotencyKey
	return d
}

// itoa avoids pulling strconv into the test surface for a single use; lifted
// verbatim from amadeus's helpers to keep the copy-sync mirror tight.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	negative := false
	if n < 0 {
		negative = true
		n = -n
	}
	var digits [20]byte
	i := len(digits)
	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}
	if negative {
		i--
		digits[i] = '-'
	}
	return string(digits[i:])
}

func TestProjectCurrentContracts_HighestRevisionWins(t *testing.T) {
	// given two revisions of the same contract: revision 1 (older) and
	// revision 2 (newer). The newer revision must win.
	bodyV1 := readFixture(t, "valid-v1.md")
	bodyV2 := readFixture(t, "valid-v1.md")
	older := rivalDMail("spec-auth_aaaaaaaa", "auth-x", 1, "", bodyV1)
	newer := rivalDMail("spec-auth_bbbbbbbb", "auth-x", 2, "spec-auth_aaaaaaaa", bodyV2)

	// when
	current, conflicts := filter.ProjectCurrentContracts([]domain.DMail{older, newer})

	// then
	if len(conflicts) != 0 {
		t.Fatalf("expected no conflicts, got %+v", conflicts)
	}
	if len(current) != 1 {
		t.Fatalf("expected 1 current contract, got %d", len(current))
	}
	if current[0].Metadata.Revision != 2 {
		t.Errorf("expected winning revision 2, got %d", current[0].Metadata.Revision)
	}
	if current[0].DMailName != "spec-auth_bbbbbbbb" {
		t.Errorf("expected winner name 'spec-auth_bbbbbbbb', got %q", current[0].DMailName)
	}
}

func TestProjectCurrentContracts_DuplicateDeliveryTolerated(t *testing.T) {
	// given the same logical contract delivered twice (e.g. through phonewave
	// at-least-once delivery). The two D-Mails share the same idempotency_key
	// and the same body. The projection must collapse them to one winner
	// without emitting a conflict.
	body := readFixture(t, "valid-v1.md")
	first := rivalDMailWithKey("spec-auth_aaaaaaaa", "auth-x", 1, "", body, "key-shared")
	second := rivalDMailWithKey("spec-auth_bbbbbbbb", "auth-x", 1, "", body, "key-shared")

	// when
	current, conflicts := filter.ProjectCurrentContracts([]domain.DMail{first, second})

	// then
	if len(conflicts) != 0 {
		t.Fatalf("expected no conflicts for duplicate delivery, got %+v", conflicts)
	}
	if len(current) != 1 {
		t.Fatalf("expected 1 current contract, got %d", len(current))
	}
	// Deterministic tie-break: lexicographically smallest D-Mail name wins.
	if current[0].DMailName != "spec-auth_aaaaaaaa" {
		t.Errorf("expected deterministic winner 'spec-auth_aaaaaaaa', got %q", current[0].DMailName)
	}
}

func TestProjectCurrentContracts_SameRevisionConflict(t *testing.T) {
	// given two D-Mails with the same contract_id and revision but different
	// bodies. The projection must emit a same-revision conflict.
	bodyA := readFixture(t, "conflicting-revision-a.md")
	bodyB := readFixture(t, "conflicting-revision-b.md")
	a := rivalDMail("spec-auth_aaaaaaaa", "auth-x", 1, "", bodyA)
	b := rivalDMail("spec-auth_bbbbbbbb", "auth-x", 1, "", bodyB)

	// when
	current, conflicts := filter.ProjectCurrentContracts([]domain.DMail{a, b})

	// then
	if len(current) != 0 {
		t.Errorf("expected no current contract while in conflict, got %+v", current)
	}
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d (%+v)", len(conflicts), conflicts)
	}
	c := conflicts[0]
	if c.ContractID != "auth-x" {
		t.Errorf("ContractID: got %q", c.ContractID)
	}
	if !strings.Contains(c.Reason, "same-revision") {
		t.Errorf("Reason: expected 'same-revision', got %q", c.Reason)
	}
	if len(c.Names) != 2 {
		t.Errorf("Names: expected both D-Mail names, got %v", c.Names)
	}
}

func TestProjectCurrentContracts_InvalidSupersedesConflict(t *testing.T) {
	// given a winning revision that points at a supersedes name which does
	// not exist in the group. The projection must emit an invalid-supersedes
	// conflict and refuse to publish a current contract for that id.
	body := readFixture(t, "valid-v1.md")
	older := rivalDMail("spec-auth_aaaaaaaa", "auth-x", 1, "", body)
	newer := rivalDMail("spec-auth_bbbbbbbb", "auth-x", 2, "spec-auth_does-not-exist", body)

	// when
	current, conflicts := filter.ProjectCurrentContracts([]domain.DMail{older, newer})

	// then
	if len(current) != 0 {
		t.Errorf("expected no current contract when supersedes lineage is invalid, got %+v", current)
	}
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d (%+v)", len(conflicts), conflicts)
	}
	if conflicts[0].ContractID != "auth-x" {
		t.Errorf("ContractID: got %q", conflicts[0].ContractID)
	}
	if !strings.Contains(conflicts[0].Reason, "supersedes") {
		t.Errorf("Reason: expected 'supersedes', got %q", conflicts[0].Reason)
	}
}

func TestProjectCurrentContracts_RejectsDMailNameContractID(t *testing.T) {
	// given a D-Mail whose metadata.contract_id matches the D-Mail name
	// pattern. The metadata parser rejects it, so the projection must skip
	// the D-Mail entirely and produce neither a current contract nor a
	// conflict.
	body := readFixture(t, "valid-v1.md")
	bad := domain.DMail{
		Name: "spec-auth_aaaaaaaa",
		Kind: domain.KindSpecification,
		Body: body,
		Metadata: map[string]string{
			"contract_schema":   filter.SchemaRivalContractV1,
			"contract_id":       "spec-auth-session_a3f2b7c4",
			"contract_revision": "1",
		},
	}

	// when
	current, conflicts := filter.ProjectCurrentContracts([]domain.DMail{bad})

	// then
	if len(current) != 0 {
		t.Errorf("expected no current contract for D-Mail-name-as-id, got %+v", current)
	}
	if len(conflicts) != 0 {
		t.Errorf("expected no conflict for invalid metadata, got %+v", conflicts)
	}
}

// TestProjectCurrentContracts_BehavesLikeAmadeus is the explicit copy-sync
// regression: any future drift between sj's copy and amadeus's canonical
// implementation will surface here. The cases mirror amadeus's
// rival_contract_test.go cases byte-for-byte.
func TestProjectCurrentContracts_BehavesLikeAmadeus(t *testing.T) {
	// given the canonical "highest revision wins" archive
	body := readFixture(t, "valid-v1.md")
	older := rivalDMail("spec-auth_aaaaaaaa", "auth-x", 1, "", body)
	newer := rivalDMail("spec-auth_bbbbbbbb", "auth-x", 2, "spec-auth_aaaaaaaa", body)

	// when
	current, conflicts := filter.ProjectCurrentContracts([]domain.DMail{older, newer})

	// then: matches amadeus expectations exactly
	if len(conflicts) != 0 {
		t.Fatalf("copy-sync drift: amadeus produces 0 conflicts, sj produced %+v", conflicts)
	}
	if len(current) != 1 {
		t.Fatalf("copy-sync drift: amadeus produces 1 current, sj produced %d", len(current))
	}
	if current[0].DMailName != "spec-auth_bbbbbbbb" {
		t.Errorf("copy-sync drift: amadeus winner is 'spec-auth_bbbbbbbb', sj returned %q", current[0].DMailName)
	}
	if current[0].Metadata.Revision != 2 {
		t.Errorf("copy-sync drift: amadeus winning revision is 2, sj returned %d", current[0].Metadata.Revision)
	}
}
