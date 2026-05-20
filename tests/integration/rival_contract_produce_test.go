// Package integration_test rival_contract_produce_test.go: Producer-side
// integration test for Rival Contract v1 D-Mails.
//
// This test calls the real `ComposeSpecification` producer in `wave` mode
// with a fully reproducible synthetic wave, reads the resulting on-disk
// outbox file, and compares it byte-for-byte against a committed golden
// fixture. The golden file at
//
//	tests/integration/testdata/rival/produced/canonical-spec-v1.md
//
// is the single source of truth for the cross-tool round-trip in Phase
// 1.2A: pt/am/dom each commit a byte-identical copy under their own
// `tests/integration/testdata/rival/canonical-spec-v1.md`. A regression
// in `ComposeSpecification` therefore breaks both this test (locally)
// and any consumer that reparses the same fixture.
//
// Determinism: `ComposeSpecification` uses an unexported uuidFunc seam
// (`internal/session/export_test.go:SetDMailUUID`) to assign a random 8
// hex character suffix to the D-Mail name. The seam is only visible
// inside the session test binary, not from this `tests/integration/`
// package, so this test instead canonicalizes the produced bytes via
// the public `session.ParseDMail` + `session.MarshalDMail` round-trip:
// the random uuid in `mail.Name` is replaced by a fixed canonical
// suffix, and `MarshalDMail` re-derives the content-hash idempotency
// key against the canonical name. Every other field in the rendered
// body and the four required Rival Contract metadata keys
// (`contract_schema`, `contract_id`, `contract_revision`, `supersedes`)
// is fully deterministic given the synthetic wave, so the
// canonicalized output is stable across runs.
//
// Maintenance: pass `-update` to regenerate the golden after an
// intentional rendering change. Re-run gap-check
// `check_rival_canonical_fixture.sh` (added in Phase 1.2A step 3) to
// re-sync pt/am/dom copies.
//
// Refs:
//   - refs/plans/2026-05-03-rival-contract-v1-2-integration-e2e.md (Phase 1.2A)
//   - refs/plans/2026-05-03-rival-contract-v1.md (canonical body+metadata)
//   - refs/plans/2026-05-03-rival-contract-v1-1-extensions.md (domain_style)
package integration_test

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/session"
)

// updateGolden regenerates the produced golden fixture when set. Pass
// `-update` (no value) to `go test` after intentional rendering changes
// or producer behavior changes are landed.
var updateGolden = flag.Bool("update", false, "regenerate Rival Contract v1 producer golden fixtures")

// canonicalSpecGoldenPath is the path to the SoT golden, relative to
// the integration test package directory.
const canonicalSpecGoldenPath = "testdata/rival/produced/canonical-spec-v1.md"

// canonicalDMailUUID is the deterministic uuid8 substituted into the
// produced D-Mail name before golden comparison. The value is arbitrary;
// it just has to be a valid 8 hex character suffix and recognizably
// fixed so reviewers do not misread it as a real random id.
const canonicalDMailUUID = "deadbeef"

// dmailUUIDPattern matches the canonical D-Mail name suffix
// `_[0-9a-f]{8}` produced by `session.shortUUID`.
var dmailUUIDPattern = regexp.MustCompile(`_[0-9a-f]{8}$`)

// canonicalSpecWave returns the synthetic wave used to produce the SoT
// canonical-spec-v1 fixture. The wave is intentionally minimal but
// exercises every Rival Contract v1 section:
//
//	# Contract: Add session expiry enforcement
//	## Intent       — wave.Description plus the renderer's two boilerplate bullets
//	## Domain       — ClusterName + IssueIDs + dispatch grammar
//	## Decisions    — placeholder (compose path provides no DiscussionSummary)
//	## Steps        — one implementation action
//	## Boundaries   — one issue-management action filtered into Boundaries
//	## Evidence     — canonical test:/lint: bullets + per-acceptance bullet
//
// The chosen names are repository-relative and contain no absolute
// paths so the body satisfies invariant 9 of the v1 plan.
func canonicalSpecWave() domain.Wave {
	return domain.Wave{
		ID:          "canonical-spec-v1",
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
			{
				Type:        "add_dod",
				IssueID:     "MY-1",
				Description: "Document expiry enforcement in DoD",
				Detail:      "Capture the new behavior in the cluster's Definition of Done.",
			},
		},
	}
}

// produceCanonicalSpecBytes runs the real `ComposeSpecification` producer
// against `canonicalSpecWave`, locates the resulting outbox D-Mail, and
// returns the canonicalized bytes (random uuid replaced by
// canonicalDMailUUID, idempotency_key recomputed against the canonical
// name via the public marshaler). The returned bytes are byte-stable
// across runs and are the SoT for the cross-tool consumer fixture.
func produceCanonicalSpecBytes(t *testing.T) []byte {
	t.Helper()

	dir := t.TempDir()
	if err := session.EnsureMailDirs(dir); err != nil {
		t.Fatalf("EnsureMailDirs: %v", err)
	}
	store, err := session.NewOutboxStoreForDir(dir)
	if err != nil {
		t.Fatalf("NewOutboxStoreForDir: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	wave := canonicalSpecWave()
	if err := session.ComposeSpecification(context.Background(), store, wave, domain.ModeWave); err != nil {
		t.Fatalf("ComposeSpecification: %v", err)
	}

	outboxGlob := filepath.Join(domain.MailDir(dir, domain.OutboxDir), "sj-spec-*.md")
	matches, err := filepath.Glob(outboxGlob)
	if err != nil {
		t.Fatalf("glob outbox: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected exactly 1 spec D-Mail in outbox, got %d (glob=%s, matches=%v)", len(matches), outboxGlob, matches)
	}
	produced, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read produced spec: %v", err)
	}

	mail, err := session.ParseDMail(produced)
	if err != nil {
		t.Fatalf("parse produced spec: %v", err)
	}

	// Canonicalize the D-Mail name by replacing the random uuid suffix
	// with canonicalDMailUUID. The exported MarshalDMail will then
	// re-derive idempotency_key against the canonical name + body, so
	// the resulting bytes are deterministic across runs.
	if !dmailUUIDPattern.MatchString(mail.Name) {
		t.Fatalf("produced D-Mail name %q does not match expected `_[0-9a-f]{8}$` suffix; producer name format changed", mail.Name)
	}
	mail.Name = dmailUUIDPattern.ReplaceAllString(mail.Name, "_"+canonicalDMailUUID)

	canonical, err := session.MarshalDMail(mail)
	if err != nil {
		t.Fatalf("marshal canonical: %v", err)
	}
	return canonical
}

// TestRivalContractProduce_ComposeSpecificationGoldenMatch is the SoT
// producer test for Phase 1.2A. It writes the same bytes that pt/am/dom
// will commit byte-identically into their own integration testdata.
func TestRivalContractProduce_ComposeSpecificationGoldenMatch(t *testing.T) {
	t.Skip("Integration test exercises ClaudeAdapter.RunDetailed deprecated post jun15 MCP pivot (refs/issues/0027); sub-B will fully delete this test")
	produced := produceCanonicalSpecBytes(t)

	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(canonicalSpecGoldenPath), 0o755); err != nil {
			t.Fatalf("mkdir golden dir: %v", err)
		}
		if err := os.WriteFile(canonicalSpecGoldenPath, produced, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("updated golden: %s (%d bytes)", canonicalSpecGoldenPath, len(produced))
		return
	}

	want, err := os.ReadFile(canonicalSpecGoldenPath)
	if err != nil {
		t.Fatalf("read golden %s: %v (run with -update to create)", canonicalSpecGoldenPath, err)
	}
	if string(produced) != string(want) {
		t.Errorf("produced spec does not match golden %s\n--- produced (first 600 bytes) ---\n%s\n--- want (first 600 bytes) ---\n%s\nrun `go test -run TestRivalContractProduce_ComposeSpecificationGoldenMatch ./tests/integration/... -update` after intentional rendering changes",
			canonicalSpecGoldenPath, firstNBytes(produced, 600), firstNBytes(want, 600))
	}
}

// TestRivalContractProduce_GoldenSatisfiesParser asserts the on-disk
// golden parses cleanly via the canonical parser. This is a cheap
// regression net for accidental edits to the committed fixture: even
// without rerunning the producer, anyone who edits the golden by hand
// must keep it parseable.
func TestRivalContractProduce_GoldenSatisfiesParser(t *testing.T) {
	data, err := os.ReadFile(canonicalSpecGoldenPath)
	if err != nil {
		t.Fatalf("read golden %s: %v (the producer test must have written it first; run with -update if missing)", canonicalSpecGoldenPath, err)
	}
	mail, err := session.ParseDMail(data)
	if err != nil {
		t.Fatalf("ParseDMail golden: %v", err)
	}
	if mail.Kind != domain.KindSpecification {
		t.Errorf("golden Kind: got %q, want %q", mail.Kind, domain.KindSpecification)
	}
	if mail.SchemaVersion != domain.DMailSchemaVersion {
		t.Errorf("golden SchemaVersion: got %q, want %q", mail.SchemaVersion, domain.DMailSchemaVersion)
	}
	if !strings.HasSuffix(mail.Name, "_"+canonicalDMailUUID) {
		t.Errorf("golden Name must end in canonical uuid suffix `_%s`, got %q", canonicalDMailUUID, mail.Name)
	}
	// Body must declare the canonical title.
	if !strings.HasPrefix(strings.TrimLeft(mail.Body, "\n"), "# Contract: Add session expiry enforcement") {
		t.Errorf("golden body must start with `# Contract: Add session expiry enforcement`, got first 80 bytes: %q", firstNBytes([]byte(mail.Body), 80))
	}
}

// firstNBytes returns the first n bytes of b for diagnostic messages.
func firstNBytes(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n])
}
