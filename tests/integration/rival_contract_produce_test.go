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
	"os"
	"strings"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/session"
)

// canonicalSpecGoldenPath is the path to the SoT golden, relative to
// the integration test package directory.
const canonicalSpecGoldenPath = "testdata/rival/produced/canonical-spec-v1.md"

// canonicalDMailUUID is the deterministic uuid8 substituted into the
// produced D-Mail name before golden comparison. The value is arbitrary;
// it just has to be a valid 8 hex character suffix and recognizably
// fixed so reviewers do not misread it as a real random id.
const canonicalDMailUUID = "deadbeef"

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
