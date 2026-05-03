// Package integration_test rival_amendment_extract_test.go: black-box
// consumer-side integration test for the Rival Contract v1 amendment
// loop (Phase 1.2B sj side).
//
// This test consumes the byte-identical copy of amadeus's emitted
// design-feedback golden:
//
//	tests/integration/testdata/rival/cross-tool/amadeus-emitted-correction.md
//
// (sourced from /Users/nino/tap/amadeus/internal/session/testdata/rival/
// cross-tool/amadeus-emitted-correction.md — drift between the two
// copies is guarded by the `check_rival_amendment_fixture.sh` gap-check
// at the tap monorepo root). The test then runs sightjack's real
// `ExtractContractAmendments` and `ComposeAmendedSpecification` against
// it to verify the cross-tool amendment cycle: amadeus emits feedback
// with a canonical `## Contract Amendments` block → sightjack parses it
// → sightjack composes an amended specification D-Mail at revision +1
// pointing back at the previous via `supersedes`, leaving the previous
// file byte-for-byte unchanged.
//
// Determinism: this test does NOT depend on D-Mail name byte-stability
// — assertions target only the parsed metadata fields
// (`contract_revision`, `contract_id`, `supersedes`) and the sha256 of
// the previous on-disk file. The `session.SetDMailUUID` seam in
// `internal/session/export_test.go` is intentionally NOT reused here:
// it lives in the session package's test binary and is invisible from
// this `integration_test` package, which is exactly the black-box
// boundary we want.
//
// Refs:
//   - refs/plans/2026-05-03-rival-contract-v1-2-integration-e2e.md
//     (Phase 1.2B step B — sj-side extract+compose)
//   - refs/plans/2026-05-03-rival-contract-v1.md (Phase 5: amendment loop)
package integration_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/harness"
	"github.com/hironow/sightjack/internal/session"
)

// amendmentFeedbackFixturePath is the path to the byte-identical copy
// of amadeus's emitted correction D-Mail body. The same bytes live
// (byte-for-byte) at amadeus/internal/session/testdata/rival/cross-tool/
// amadeus-emitted-correction.md — drift is detected by
// refs/scripts/check_rival_amendment_fixture.sh.
const amendmentFeedbackFixturePath = "testdata/rival/cross-tool/amadeus-emitted-correction.md"

// amendmentFixtureWave returns the synthetic wave used as the lineage
// anchor for the amended-specification compose. The ID matches the
// contract id encoded in the fixture's "- Contract: `auth-session-expiry`"
// bullet so the amendments are applied to a contract whose id is stable.
func amendmentFixtureWave() domain.Wave {
	return domain.Wave{
		ID:          "auth-session-expiry",
		ClusterName: "auth",
		Title:       "Add session expiry enforcement",
		Description: "Reject expired sessions at the auth middleware before the request reaches business logic.",
		Actions: []domain.WaveAction{
			{
				Type:        "implement",
				IssueID:     "MY-1",
				Description: "Enforce session expiry in middleware",
				Detail:      "Update the auth middleware to read the expiry claim and reject expired tokens with a 401.",
			},
		},
	}
}

// composeBaseRevision composes a Rival Contract v1 specification D-Mail
// for the test wave at revision 1, and rewrites its on-disk
// `contract_revision` to `revision` when revision > 1 so the amendment
// path can target an arbitrary previous revision. The returned values
// identify the previous D-Mail (logical name, on-disk path, parsed
// metadata) — they are the inputs `ComposeAmendedSpecification` needs.
//
// This mirrors the white-box helper of the same name in
// internal/session/rival_contract_amendment_test.go but uses only the
// public session API surface (`ComposeSpecification`,
// `ParseDMail`/`MarshalDMail`, `harness.ParseRivalContractMetadata`).
func composeBaseRevision(t *testing.T, dir string, store *session.SQLiteOutboxStore, wave domain.Wave, revision int) (name, path string, meta harness.RivalContractMetadata) {
	t.Helper()
	if err := session.ComposeSpecification(context.Background(), store, wave, domain.ModeWave); err != nil {
		t.Fatalf("ComposeSpecification (base rev=%d): %v", revision, err)
	}
	matches, _ := filepath.Glob(filepath.Join(domain.MailDir(dir, domain.OutboxDir), "sj-spec-*.md"))
	if len(matches) != 1 {
		t.Fatalf("expected 1 spec D-Mail in outbox, got %d", len(matches))
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read base spec: %v", err)
	}
	mail, err := session.ParseDMail(data)
	if err != nil {
		t.Fatalf("parse base spec: %v", err)
	}
	if revision > 1 {
		mail.Metadata["contract_revision"] = strconv.Itoa(revision)
		out, err := session.MarshalDMail(mail)
		if err != nil {
			t.Fatalf("re-marshal base spec: %v", err)
		}
		if err := os.WriteFile(matches[0], out, 0o644); err != nil {
			t.Fatalf("rewrite base spec: %v", err)
		}
	}
	parsed, ok, err := harness.ParseRivalContractMetadata(mail.Metadata)
	if err != nil || !ok {
		t.Fatalf("ParseRivalContractMetadata: ok=%v err=%v meta=%v", ok, err, mail.Metadata)
	}
	return mail.Name, matches[0], parsed
}

// newAmendmentStore opens a fresh outbox store for `dir` and registers
// Close on cleanup, matching the white-box helper but using only the
// public API surface.
func newAmendmentStore(t *testing.T, dir string) *session.SQLiteOutboxStore {
	t.Helper()
	if err := session.EnsureMailDirs(dir); err != nil {
		t.Fatalf("EnsureMailDirs: %v", err)
	}
	store, err := session.NewOutboxStoreForDir(dir)
	if err != nil {
		t.Fatalf("create outbox store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// TestRivalAmendmentExtract_FromFeedbackBody asserts the deterministic
// extractor reads the canonical amadeus-emitted golden body and returns
// the amendments encoded in its `## Contract Amendments` section in
// document order.
func TestRivalAmendmentExtract_FromFeedbackBody(t *testing.T) {
	// given: the byte-identical copy of amadeus's emitted correction body.
	bodyBytes, err := os.ReadFile(amendmentFeedbackFixturePath)
	if err != nil {
		t.Fatalf("read amendment fixture %s: %v", amendmentFeedbackFixturePath, err)
	}

	// when: the deterministic extractor parses the body.
	amendments, err := session.ExtractContractAmendments(string(bodyBytes))
	if err != nil {
		t.Fatalf("ExtractContractAmendments: %v", err)
	}

	// then: two amendments are returned in document order with the
	// exact fields parsed out of the bullet grammar in the fixture.
	if got := len(amendments); got != 2 {
		t.Fatalf("amendment count: got %d, want 2 (amendments=%+v)", got, amendments)
	}
	for i, a := range amendments {
		if a.ContractID != "auth-session-expiry" {
			t.Errorf("amendments[%d].ContractID: got %q, want %q", i, a.ContractID, "auth-session-expiry")
		}
	}
	// First bullet: Boundaries amendment with rationale.
	if amendments[0].Section != "Boundaries" {
		t.Errorf("amendments[0].Section: got %q, want %q", amendments[0].Section, "Boundaries")
	}
	if !strings.Contains(amendments[0].Suggestion, "OAuth refresh tokens") {
		t.Errorf("amendments[0].Suggestion missing expected text: got %q", amendments[0].Suggestion)
	}
	if !strings.Contains(amendments[0].Rationale, "SSO requires refresh") {
		t.Errorf("amendments[0].Rationale missing expected text: got %q", amendments[0].Rationale)
	}
	// Suggestion must not contain rationale segment.
	if strings.Contains(amendments[0].Suggestion, "rationale:") {
		t.Errorf("amendments[0].Suggestion must not contain rationale segment: %q", amendments[0].Suggestion)
	}
	// Second bullet: Evidence amendment without rationale.
	if amendments[1].Section != "Evidence" {
		t.Errorf("amendments[1].Section: got %q, want %q", amendments[1].Section, "Evidence")
	}
	if !strings.Contains(amendments[1].Suggestion, "nfr.p95_latency_ms") {
		t.Errorf("amendments[1].Suggestion missing nfr key: got %q", amendments[1].Suggestion)
	}
	if amendments[1].Rationale != "" {
		t.Errorf("amendments[1].Rationale should be empty, got %q", amendments[1].Rationale)
	}
}

// TestRivalAmendmentExtract_AmendedComposeIncrementsRevision asserts
// that feeding the extracted amendments + a synthetic previous spec
// (revision 2) to `ComposeAmendedSpecification` produces a new D-Mail
// whose metadata.contract_revision is exactly previous + 1 = 3.
func TestRivalAmendmentExtract_AmendedComposeIncrementsRevision(t *testing.T) {
	// given: a synthetic previous Rival Contract v1 spec at revision 2.
	dir := t.TempDir()
	store := newAmendmentStore(t, dir)
	wave := amendmentFixtureWave()
	prevName, _, prevMeta := composeBaseRevision(t, dir, store, wave, 2)

	// and: the amendments extracted from amadeus's emitted golden.
	bodyBytes, err := os.ReadFile(amendmentFeedbackFixturePath)
	if err != nil {
		t.Fatalf("read amendment fixture: %v", err)
	}
	amendments, err := session.ExtractContractAmendments(string(bodyBytes))
	if err != nil {
		t.Fatalf("ExtractContractAmendments: %v", err)
	}

	// when: the amended-specification compose path is invoked.
	feedbackName := "feedback-spec-auth_aabbccdd"
	amended, err := session.ComposeAmendedSpecification(
		context.Background(), store, wave,
		prevMeta, prevName,
		amendments, feedbackName, domain.ModeWave,
	)
	if err != nil {
		t.Fatalf("ComposeAmendedSpecification: %v", err)
	}

	// then: contract_revision = previous + 1 = 3, and contract_id
	// (lineage anchor) is preserved across the amendment.
	if got := amended.Metadata["contract_revision"]; got != "3" {
		t.Errorf("contract_revision: got %q, want %q", got, "3")
	}
	if got := amended.Metadata["contract_id"]; got != "auth-session-expiry" {
		t.Errorf("contract_id: got %q, want %q", got, "auth-session-expiry")
	}
}

// TestRivalAmendmentExtract_AmendedComposeSetsSupersedes asserts that
// the amended D-Mail's metadata.supersedes equals the previous D-Mail
// name passed in, closing the lineage link the amadeus archive scan
// will follow on the next pass.
func TestRivalAmendmentExtract_AmendedComposeSetsSupersedes(t *testing.T) {
	// given: a previous spec D-Mail (revision 1) on disk.
	dir := t.TempDir()
	store := newAmendmentStore(t, dir)
	wave := amendmentFixtureWave()
	prevName, _, prevMeta := composeBaseRevision(t, dir, store, wave, 1)

	bodyBytes, err := os.ReadFile(amendmentFeedbackFixturePath)
	if err != nil {
		t.Fatalf("read amendment fixture: %v", err)
	}
	amendments, err := session.ExtractContractAmendments(string(bodyBytes))
	if err != nil {
		t.Fatalf("ExtractContractAmendments: %v", err)
	}

	// when: the amended path runs against that previous revision.
	feedbackName := "feedback-spec-auth_deadbeef"
	amended, err := session.ComposeAmendedSpecification(
		context.Background(), store, wave,
		prevMeta, prevName,
		amendments, feedbackName, domain.ModeWave,
	)
	if err != nil {
		t.Fatalf("ComposeAmendedSpecification: %v", err)
	}

	// then: supersedes is exactly the previous D-Mail name.
	if got := amended.Metadata["supersedes"]; got != prevName {
		t.Errorf("supersedes: got %q, want %q", got, prevName)
	}
	// and: the body cites the feedback D-Mail name so reviewers can
	// trace the amendment back to its source.
	if !strings.Contains(amended.Body, feedbackName) {
		t.Errorf("amended body should cite feedback D-Mail name %q in body, got:\n%s", feedbackName, amended.Body)
	}
}

// TestRivalAmendmentExtract_AmendedComposePreviousFileUnchanged asserts
// that the previous D-Mail file on disk is byte-for-byte unchanged
// after the amended-spec compose runs. The amendment loop is strictly
// append-only by design: a new revision is published as a NEW file and
// the predecessor's bytes must not be mutated.
func TestRivalAmendmentExtract_AmendedComposePreviousFileUnchanged(t *testing.T) {
	// given: a previous spec D-Mail file on disk.
	dir := t.TempDir()
	store := newAmendmentStore(t, dir)
	wave := amendmentFixtureWave()
	prevName, prevPath, prevMeta := composeBaseRevision(t, dir, store, wave, 1)

	// and: a sha256 baseline of the previous file's bytes.
	beforeBytes, err := os.ReadFile(prevPath)
	if err != nil {
		t.Fatalf("read previous spec: %v", err)
	}
	beforeSum := sha256.Sum256(beforeBytes)

	// and: amendments parsed from amadeus's emitted golden.
	bodyBytes, err := os.ReadFile(amendmentFeedbackFixturePath)
	if err != nil {
		t.Fatalf("read amendment fixture: %v", err)
	}
	amendments, err := session.ExtractContractAmendments(string(bodyBytes))
	if err != nil {
		t.Fatalf("ExtractContractAmendments: %v", err)
	}

	// when: the amended path runs.
	if _, err := session.ComposeAmendedSpecification(
		context.Background(), store, wave,
		prevMeta, prevName,
		amendments, "feedback-spec-auth_deadbeef", domain.ModeWave,
	); err != nil {
		t.Fatalf("ComposeAmendedSpecification: %v", err)
	}

	// then: the previous D-Mail file's sha256 is unchanged.
	afterBytes, err := os.ReadFile(prevPath)
	if err != nil {
		t.Fatalf("re-read previous spec: %v", err)
	}
	afterSum := sha256.Sum256(afterBytes)
	if beforeSum != afterSum {
		t.Errorf("previous D-Mail file MUST NOT be modified; sha256 differs:\n  before=%s\n  after=%s",
			hex.EncodeToString(beforeSum[:]), hex.EncodeToString(afterSum[:]))
	}
}
