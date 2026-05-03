// white-box-reason: tests session-level deterministic Rival Contract v1
// amendment extractor and amended-specification compose path. Both
// extraction (no LLM) and the produce side need session-package access
// to keep the lineage invariants close to the producer.
package session

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/harness"
)

func TestExtractContractAmendments_FromDesignFeedback(t *testing.T) {
	// given a design-feedback body in the exact shape amadeus Phase 3
	// emits (see /Users/nino/tap/amadeus/internal/session/rival_contract_correction.go):
	//   - leading "## Contract Amendments" heading
	//   - "- Contract: `<id>` (revision N)" identity bullet
	//   - "- Amend <Section>: <Suggestion> (rationale: <Rationale>)" bullets
	body := strings.Join([]string{
		"Boundaries section is no longer reflective of SSO requirements.",
		"",
		"## Contract Amendments",
		"",
		"- Contract: `auth-session-expiry` (revision 2)",
		"- Amend Boundaries: Allow short-lived OAuth refresh tokens for first-party clients. (rationale: Implementation now requires SSO; original boundary is obsolete.)",
		"- Amend Evidence: Add nfr.p95_latency_ms: <= 250 to reflect SSO overhead.",
		"- Amend (unspecified): Clarify ambiguous wording around session lifetimes.",
		"",
	}, "\n")

	// when the deterministic extractor parses the body
	amendments, err := ExtractContractAmendments(body)

	// then three amendments are returned in document order with the
	// exact fields parsed out of the bullet grammar.
	if err != nil {
		t.Fatalf("ExtractContractAmendments: %v", err)
	}
	if got := len(amendments); got != 3 {
		t.Fatalf("amendment count: got %d, want 3 (amendments=%+v)", got, amendments)
	}

	// All amendments must carry the contract id from the leading bullet.
	for i, a := range amendments {
		if a.ContractID != "auth-session-expiry" {
			t.Errorf("amendment[%d] ContractID: got %q, want %q", i, a.ContractID, "auth-session-expiry")
		}
	}

	// First bullet: Boundaries amendment with rationale.
	if amendments[0].Section != "Boundaries" {
		t.Errorf("amendments[0].Section: got %q, want %q", amendments[0].Section, "Boundaries")
	}
	if !strings.Contains(amendments[0].Suggestion, "OAuth refresh tokens for first-party clients") {
		t.Errorf("amendments[0].Suggestion missing expected text: got %q", amendments[0].Suggestion)
	}
	if !strings.Contains(amendments[0].Rationale, "original boundary is obsolete") {
		t.Errorf("amendments[0].Rationale missing expected text: got %q", amendments[0].Rationale)
	}
	// Suggestion must NOT contain the rationale segment.
	if strings.Contains(amendments[0].Suggestion, "rationale:") {
		t.Errorf("amendments[0].Suggestion must not contain rationale segment: %q", amendments[0].Suggestion)
	}

	// Second bullet: Evidence amendment without rationale.
	if amendments[1].Section != "Evidence" {
		t.Errorf("amendments[1].Section: got %q", amendments[1].Section)
	}
	if !strings.Contains(amendments[1].Suggestion, "nfr.p95_latency_ms") {
		t.Errorf("amendments[1].Suggestion missing nfr key: got %q", amendments[1].Suggestion)
	}
	if amendments[1].Rationale != "" {
		t.Errorf("amendments[1].Rationale should be empty, got %q", amendments[1].Rationale)
	}

	// Third bullet: (unspecified) section pass-through (do NOT drop it; the
	// nextgen LLM still gets a chance to infer the target section).
	if amendments[2].Section != "(unspecified)" {
		t.Errorf("amendments[2].Section: got %q, want %q", amendments[2].Section, "(unspecified)")
	}
}

func TestExtractContractAmendments_NoAmendmentsSectionReturnsEmpty(t *testing.T) {
	// given a feedback body with NO "## Contract Amendments" section.
	body := strings.Join([]string{
		"Some legacy feedback body.",
		"",
		"## Other Heading",
		"- random bullet",
	}, "\n")

	// when
	amendments, err := ExtractContractAmendments(body)

	// then no amendments and no error.
	if err != nil {
		t.Fatalf("ExtractContractAmendments unexpected error: %v", err)
	}
	if len(amendments) != 0 {
		t.Errorf("expected 0 amendments, got %d: %+v", len(amendments), amendments)
	}
}

func TestExtractContractAmendments_SkipsNonMatchingBulletsSilently(t *testing.T) {
	// given a body that has the section but the bullets are mostly noise:
	// only the canonical "- Amend <Section>: ..." grammar should be picked.
	body := strings.Join([]string{
		"## Contract Amendments",
		"",
		"- Contract: `c-1`",
		"- Amend Intent: Reword for clarity.",
		"- not a bullet, skip",
		"- Just some prose. No 'Amend ' prefix.",
		"- Amend: Missing section name.",
		"- Amend Steps: Add step 3.",
	}, "\n")

	// when
	amendments, err := ExtractContractAmendments(body)

	// then only the two well-formed bullets are kept; the malformed
	// "Amend: Missing section name." bullet is silently dropped because
	// its grammar does not match.
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got := len(amendments); got != 2 {
		t.Fatalf("expected 2 amendments, got %d: %+v", got, amendments)
	}
	if amendments[0].Section != "Intent" {
		t.Errorf("amendments[0].Section: got %q", amendments[0].Section)
	}
	if amendments[1].Section != "Steps" {
		t.Errorf("amendments[1].Section: got %q", amendments[1].Section)
	}
	for i, a := range amendments {
		if a.ContractID != "c-1" {
			t.Errorf("amendments[%d].ContractID: got %q, want %q", i, a.ContractID, "c-1")
		}
	}
}

func TestNextGenPrompt_IncludesContractAmendments(t *testing.T) {
	// given a NextGenPromptData populated with a non-empty amendments
	// section (the new field added in Phase 5).
	data := domain.NextGenPromptData{
		ClusterName:     "auth",
		Completeness:    "55",
		Issues:          "[]",
		CompletedWaves:  "[]",
		StrictnessLevel: "alert",
		OutputPath:      "/tmp/out.json",
		// new: amendments rendered into a dedicated section
		AmendmentsSection: strings.Join([]string{
			"### feedback-spec-auth_a1b2c3d4",
			"- Contract: `auth-session-expiry` (revision 2)",
			"- Amend Boundaries: Allow short-lived OAuth refresh tokens. (rationale: SSO requirement.)",
		}, "\n"),
	}

	// when the harness renders the nextgen prompt for the English locale
	got, err := harness.RenderNextGenPrompt("en", data)

	// then the prompt body contains the amendments section text under a
	// canonical heading so the LLM can react to it.
	if err != nil {
		t.Fatalf("RenderNextGenPrompt: %v", err)
	}
	if !strings.Contains(got, "Contract Amendments") {
		t.Errorf("nextgen prompt should contain a 'Contract Amendments' heading, got body:\n%s", got)
	}
	if !strings.Contains(got, "auth-session-expiry") {
		t.Errorf("nextgen prompt should cite contract id, got body:\n%s", got)
	}
	if !strings.Contains(got, "OAuth refresh tokens") {
		t.Errorf("nextgen prompt should include amendment suggestion, got body:\n%s", got)
	}

	// ja locale also gets the amendments under a translated heading.
	gotJa, err := harness.RenderNextGenPrompt("ja", data)
	if err != nil {
		t.Fatalf("RenderNextGenPrompt(ja): %v", err)
	}
	if !strings.Contains(gotJa, "auth-session-expiry") {
		t.Errorf("nextgen prompt (ja) should cite contract id")
	}
}

func TestComposeAmendedSpecification_IncrementsRevision(t *testing.T) {
	// given a previous Rival Contract v1 specification D-Mail at revision 2.
	dir := t.TempDir()
	store := newAmendmentTestStore(t, dir)
	wave := domain.Wave{
		ID:          "auth-session-expiry",
		ClusterName: "auth",
		Title:       "Add session expiry enforcement",
		Description: "Prevent expired sessions from authorizing API calls.",
		Actions: []domain.WaveAction{
			{Type: "implement", IssueID: "MY-1", Description: "Add expiry check"},
		},
	}
	previous := composeBaseRevisionForAmendmentTest(t, dir, store, wave, 2)
	amendments := []RivalContractAmendment{
		{
			ContractID: "auth-session-expiry",
			Section:    "Boundaries",
			Suggestion: "Allow short-lived OAuth refresh tokens for first-party clients.",
			Rationale:  "SSO requires refresh.",
		},
	}

	// when the amended-specification compose path is invoked with the
	// previous current-D-Mail name, contract id, and revision
	feedbackName := "feedback-spec-auth_aabbccdd"
	amended, err := ComposeAmendedSpecification(
		context.Background(), store, wave,
		previous.metadata, previous.name,
		amendments, feedbackName, domain.ModeWave,
	)
	if err != nil {
		t.Fatalf("ComposeAmendedSpecification: %v", err)
	}

	// then the amended D-Mail has revision = previous + 1.
	if got := amended.Metadata["contract_revision"]; got != "3" {
		t.Errorf("contract_revision: got %q, want %q", got, "3")
	}
	if got := amended.Metadata["contract_id"]; got != "auth-session-expiry" {
		t.Errorf("contract_id: got %q, want %q", got, "auth-session-expiry")
	}
}

func TestComposeAmendedSpecification_SetsSupersedes(t *testing.T) {
	// given a previous Rival Contract v1 specification D-Mail.
	dir := t.TempDir()
	store := newAmendmentTestStore(t, dir)
	wave := domain.Wave{
		ID:          "wid-1",
		ClusterName: "core",
		Title:       "Initial title",
		Actions: []domain.WaveAction{
			{Type: "implement", IssueID: "I-1", Description: "Action"},
		},
	}
	previous := composeBaseRevisionForAmendmentTest(t, dir, store, wave, 1)
	feedbackName := "feedback-spec-core_deadbeef"

	// when the amended path runs against that previous revision
	amended, err := ComposeAmendedSpecification(
		context.Background(), store, wave,
		previous.metadata, previous.name,
		[]RivalContractAmendment{{ContractID: "wid-1", Section: "Steps", Suggestion: "Add step 2."}},
		feedbackName, domain.ModeWave,
	)
	if err != nil {
		t.Fatalf("ComposeAmendedSpecification: %v", err)
	}

	// then supersedes is exactly the previous D-Mail name.
	if got := amended.Metadata["supersedes"]; got != previous.name {
		t.Errorf("supersedes: got %q, want %q", got, previous.name)
	}
	// and the body cites the feedback D-Mail name so reviewers can trace
	// the amendment back to its source.
	if !strings.Contains(amended.Body, feedbackName) {
		t.Errorf("amended body should cite feedback D-Mail name %q in body, got:\n%s", feedbackName, amended.Body)
	}
}

func TestComposeAmendedSpecification_DoesNotModifyPreviousDMail(t *testing.T) {
	// given a previous specification D-Mail file written to outbox.
	dir := t.TempDir()
	store := newAmendmentTestStore(t, dir)
	wave := domain.Wave{
		ID:          "immut-1",
		ClusterName: "core",
		Title:       "Immutable previous",
		Actions: []domain.WaveAction{
			{Type: "implement", IssueID: "I-1", Description: "Action"},
		},
	}
	previous := composeBaseRevisionForAmendmentTest(t, dir, store, wave, 1)
	previousBytes, err := os.ReadFile(previous.path)
	if err != nil {
		t.Fatalf("read previous spec: %v", err)
	}

	// when the amended path runs
	if _, err := ComposeAmendedSpecification(
		context.Background(), store, wave,
		previous.metadata, previous.name,
		[]RivalContractAmendment{{ContractID: "immut-1", Section: "Steps", Suggestion: "Add step 2."}},
		"feedback-spec-core_aaaaaaaa", domain.ModeWave,
	); err != nil {
		t.Fatalf("ComposeAmendedSpecification: %v", err)
	}

	// then the previous D-Mail file is byte-for-byte unchanged.
	after, err := os.ReadFile(previous.path)
	if err != nil {
		t.Fatalf("re-read previous spec: %v", err)
	}
	if string(previousBytes) != string(after) {
		t.Errorf("previous D-Mail file MUST NOT be modified; before/after differ")
	}
}

// previousRevisionRecord captures the on-disk identity of a base
// specification D-Mail used as the supersedes target by amendment tests.
type previousRevisionRecord struct {
	name     string
	path     string
	metadata harness.RivalContractMetadata
}

// newAmendmentTestStore opens a fresh outbox store for the test directory
// and registers Close on cleanup.
func newAmendmentTestStore(t *testing.T, dir string) *SQLiteOutboxStore {
	t.Helper()
	if err := EnsureMailDirs(dir); err != nil {
		t.Fatalf("EnsureMailDirs: %v", err)
	}
	store, err := NewOutboxStoreForDir(dir)
	if err != nil {
		t.Fatalf("create outbox store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// composeBaseRevisionForAmendmentTest composes a Rival Contract v1
// specification D-Mail for `wave`, optionally rewriting the on-disk
// `contract_revision` to a higher value so the amendment path can be
// exercised against arbitrary starting points.
//
// Returns the previous D-Mail's logical name, on-disk path, and parsed
// metadata. The on-disk file is left readable so the immutability
// invariant test can compare bytes before/after.
func composeBaseRevisionForAmendmentTest(t *testing.T, dir string, store *SQLiteOutboxStore, wave domain.Wave, revision int) previousRevisionRecord {
	t.Helper()
	if err := ComposeSpecification(context.Background(), store, wave, domain.ModeWave); err != nil {
		t.Fatalf("ComposeSpecification (base rev=%d): %v", revision, err)
	}
	matches, _ := filepath.Glob(filepath.Join(domain.MailDir(dir, "outbox"), "sj-spec-*.md"))
	if len(matches) != 1 {
		t.Fatalf("expected 1 spec D-Mail in outbox, got %d", len(matches))
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read base spec: %v", err)
	}
	mail, err := ParseDMail(data)
	if err != nil {
		t.Fatalf("parse base spec: %v", err)
	}
	if revision > 1 {
		mail.Metadata["contract_revision"] = strconv.Itoa(revision)
		out, err := MarshalDMail(mail)
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
	return previousRevisionRecord{
		name:     mail.Name,
		path:     matches[0],
		metadata: parsed,
	}
}
