# Rival Contract v1 (sightjack — producer)

sightjack is the **producer** of Rival Contract v1 specification D-Mails.
This document describes how the contract is rendered, what metadata is
attached, and how the amendment loop revises a contract without breaking
its lineage.

The full cross-tool plan lives at
[`refs/plans/2026-05-03-rival-contract-v1.md`](../../refs/plans/2026-05-03-rival-contract-v1.md).

## What it is

A Rival Contract v1 is the canonical Markdown body of a `kind: specification`
D-Mail. It captures design intent, execution steps, boundaries, and
verification evidence in exactly six sections so downstream tools
(paintress, amadeus, dominator) can parse and act on it deterministically.

The contract intentionally lives inside the existing D-Mail v1 transport.
sightjack does not invent a new schema field — it reuses the existing
`metadata` map and Markdown body so the rest of the loop (phonewave
delivery, archive replay, idempotency) keeps working unchanged.

## Where the producer lives

| Concern | File |
|---------|------|
| ComposeSpecification (D-Mail body + metadata) | `internal/session/dmail_compose.go` |
| Rival Contract Markdown render | `internal/harness/filter/rival_contract_render.go` |
| Pure parser (round-trip safety) | `internal/harness/filter/rival_contract.go` |
| Amendment lineage extractor | `internal/session/rival_contract_amendment.go` |
| Wave -> Rival Contract prompt builder | `internal/harness/filter/prompt_builder.go` |

The producer is wired into the existing wave-mode `apply` flow and the
`nextgen` follow-up flow. No new CLI verb was added in Phase 1.

## Six canonical sections

Every Rival Contract v1 body uses exactly these headings. Section order
is fixed; the parser rejects partial bodies with missing sections.

```markdown
# Contract: <title>

## Intent
why this work exists, who benefits, what success looks like

## Domain
domain terms, events, commands, read models, ownership boundaries

## Decisions
chosen approach, rejected alternatives, trade-offs

## Steps
ordered executable work units, aligned with wave.steps

## Boundaries
norms, safeguards, non-goals, forbidden edits, capability boundaries

## Evidence
tests, static checks, reviewer expectations, NFR thresholds, acceptance signals
```

A legacy specification body (no `# Contract:` heading) parses as
`ok=false` and downstream tools fall back to legacy behavior. This is
the migration ramp.

## Required metadata

The producer attaches the following keys to the D-Mail `metadata` map.
These are how amadeus and paintress identify and project the current
revision.

```yaml
metadata:
  contract_schema: rival-contract-v1
  contract_id: "<stable work-unit id>"
  contract_revision: "1"
  supersedes: ""
```

Rules enforced in code:

- `contract_schema` is always the literal string `rival-contract-v1`.
- `contract_revision` is a decimal integer encoded as a string because
  the existing D-Mail metadata map is `map[string]string`.
- `supersedes` is empty for a first-revision contract and otherwise
  contains the immediately previous D-Mail name. Multiple superseded
  names are out of scope for v1.

## Stable contract_id rule

`contract_id` MUST be stable across revisions. The producer derives it
in this order:

1. Wave id (`wave.id`) — preferred when the contract was generated from
   a wave.
2. A stable work-unit id derived from upstream issue or cluster
   identity — used when no wave exists.

The D-Mail `name` is **never** used as `contract_id`. D-Mail names are
message identities; if we used them as contract identities the lineage
would break the moment we issued a revision.

`DeriveContractID` returns an error rather than silently falling back
to the D-Mail name.

## Amendment loop (Phase 5)

When amadeus detects drift between an implementation and its contract,
it sends a `kind: design-feedback` D-Mail with a `## Contract Amendments`
section listing the corrective deltas. sightjack's amendment loop:

1. Extracts the `## Contract Amendments` block from the inbound
   feedback (`internal/session/rival_contract_amendment.go`).
2. Loads the previous contract revision identified by `contract_id`.
3. Merges the amendments into the appropriate sections.
4. Emits a new specification D-Mail with:
   - `contract_id` preserved (lineage stays stable).
   - `contract_revision` = previous revision + 1.
   - `supersedes` = previous D-Mail name.
5. Appends a short `## Amendment Lineage` trailer to the new body so
   the chain is visible to humans reading the archived spec.

The lineage trailer is informational — amadeus uses the metadata
`supersedes` chain for projection, not the trailer text.

## Cross-tool reference

| Tool | Role | Doc |
|------|------|-----|
| sightjack | producer (you are here) | this file |
| paintress | consumer (expedition prompt) | [paintress/docs/rival-contract-v1.md](../../paintress/docs/rival-contract-v1.md) |
| amadeus | drift controller (archive projection + corrective D-Mails) | [amadeus/docs/rival-contract-v1.md](../../amadeus/docs/rival-contract-v1.md) |
| dominator | NFR judge (Evidence -> NfrConfig) | [dominator/docs/rival-contract-v1.md](../../dominator/docs/rival-contract-v1.md) |

## Plan reference

- [`refs/plans/2026-05-03-rival-contract-v1.md`](../../refs/plans/2026-05-03-rival-contract-v1.md) — full design, phase plan, risks
- [`refs/scripts/check_rival_contract_docs.sh`](../../refs/scripts/check_rival_contract_docs.sh) — gap-check enforcement

## v1.1 additions

Rival Contract v1.1 is a purely additive minor extension. The schema name
remains `rival-contract-v1` — the SCHEMA itself did not change. Only this
spec doc gets a new appendix describing two opt-in capabilities:

1. An OPTIONAL metadata key `metadata.domain_style`.
2. A new sightjack-only subcommand `sightjack rival export reasons` that
   projects a Rival Contract v1 spec into the OpenSPDD REASONS Canvas.

Plan: [`refs/plans/2026-05-03-rival-contract-v1-1-extensions.md`](../../refs/plans/2026-05-03-rival-contract-v1-1-extensions.md).

### `metadata.domain_style` (optional)

The producer MAY attach an optional `domain_style` key to the D-Mail
`metadata` map when emitting a new contract. The key takes one of three
enumerated values:

- `event-sourced` — the target subsystem is event-sourced; contributors
  SHOULD use Command / Event / Read Model / Aggregate vocabulary in the
  `## Domain` section.
- `generic` — any domain noun phrasing is acceptable; no structural
  expectation.
- `mixed` — starts as `generic`, includes some event-sourced bullets where
  applicable.

```yaml
metadata:
  contract_schema: rival-contract-v1
  contract_id: "<stable work-unit id>"
  contract_revision: "1"
  supersedes: ""
  domain_style: event-sourced  # optional; one of event-sourced|generic|mixed
```

Producer rules:

- The producer SHOULD set `domain_style: event-sourced` when the wave or a
  matching ADR (e.g. `docs/adr/NNNN-event-sourcing-mandate.md`) signals
  event sourcing for the target subsystem. This is a producer decision; the
  producer never modifies an already-emitted D-Mail.
- The parser never infers `domain_style`. Missing key always parses as the
  empty string (treated as `generic` by all consumers).
- Unknown values are rejected by `ParseRivalContractMetadata` (only the
  three enumerated values are accepted).

Legacy v1 D-Mails (no `domain_style` key) parse identically to v1 and
produce bit-identical downstream behavior.

### `sightjack rival export reasons` subcommand

The `rival export reasons` subcommand projects a Rival Contract v1 spec
into the OpenSPDD REASONS Canvas markdown shape for external prompt-
manager interoperability (Cursor, Copilot, OpenSPDD itself). The
projection is pure: deterministic, no D-Mail mutation, no LLM, no network.

```
sightjack rival export reasons \
  --input <dmail-path> | --wave <wave-id> \
  [--output <path>]                          # default: stdout
  [--format markdown|json]                   # default: markdown
  [--allow-conflict]                         # default: false
```

`--input` and `--wave` are mutually exclusive. `--wave` resolves the
current revision through the same `ProjectCurrentContracts` projection
amadeus uses; sightjack carries an internal copy of that function for
parity (see `internal/harness/filter/rival_contract.go`). On
`ContractConflict` for the requested wave, the command exits non-zero by
default; `--allow-conflict` downgrades the failure to a stderr warning and
picks the lexicographically smaller D-Mail name for best-effort export.

Mapping table (Rival Contract v1 section → REASONS Canvas section):

| Rival Contract section | REASONS Canvas section | Notes |
|---|---|---|
| `# Contract: <title>` | (header) | Used as canvas title |
| `## Intent` | `## Requirements` | 1:1 |
| `## Domain` | `## Entities` | When `domain_style=event-sourced`, may split into Commands/Events/Read Models bullets |
| `## Decisions` | `## Approach` | 1:1 |
| (derived from Steps targets) | `## Structure` | Files / components mentioned in Steps targets |
| `## Steps` | `## Operations` | 1:1 |
| `## Boundaries` | `## Norms` AND `## Safeguards` | Norms = positive style rules; Safeguards = forbidden edits / non-goals |
| `## Evidence` | `## Validation` (extra) | Canvas has no 1:1 evidence section |
| `metadata.contract_revision` + `metadata.supersedes` | `## Sync` | Deterministic source attribution line |

Sample command output:

```text
$ sightjack rival export reasons --input ./spec-auth_aaaaaaaa.md
# REASONS Canvas: Stabilize Auth Refresh Loop

## Requirements
…

## Entities
…

## Approach
…

## Operations
…

## Norms
…

## Safeguards
…

## Validation
…

## Sync
Source: D-Mail spec-auth_aaaaaaaa.md, revision 2, supersedes spec-auth_zzzzzzzz.md
```

Exit non-zero when the source D-Mail is not Rival Contract v1 (legacy raw
specifications are not exportable in v1.1; revisit in v2 if needed).

### What did NOT change

- D-Mail v1 transport schema (no new fields, no new required keys).
- The six canonical body sections (Intent, Domain, Decisions, Steps,
  Boundaries, Evidence).
- D-Mail kinds (no new kinds; the deprecated non-canonical kind from
  Phase 4 remains forbidden).
- phonewave routing / archive behavior / idempotency.

The OpenSPDD REASONS Canvas referenced here is a vocabulary alignment
target only — sightjack does NOT depend on any external prompt manager,
and the export subcommand is one-way. There is no v1.1 import path.

## v1.2 additions — integration & E2E test coverage

Rival Contract v1.2 is a test-only minor revision. The schema name remains
`rival-contract-v1` and no production code path changed in any of the four
tools. v1.2 closes the integration and end-to-end coverage gaps left by
v1 + v1.1 by adding cross-component flow verification: the producer's real
`ComposeSpecification` writes a canonical fixture, three consumers parse
the SAME bytes, and the actual `sightjack` binary is exercised through a
subprocess for the export subcommand.

Plan: [`refs/plans/2026-05-03-rival-contract-v1-2-integration-e2e.md`](../../refs/plans/2026-05-03-rival-contract-v1-2-integration-e2e.md).

### Producer integration (sj is source of truth)

`tests/integration/rival_contract_produce_test.go` calls the real
`ComposeSpecification` against a synthetic wave + ADR + DoD context. The
emitted D-Mail is normalized through the public `ParseDMail` +
`MarshalDMail` canonicalization pipeline (avoids exposing the internal
`uuidFunc` seam) and written byte-stable to
`tests/integration/testdata/rival/produced/canonical-spec-v1.md`. This
file is the **source of truth** for cross-tool consumers; pt/am/dom each
commit a byte-identical copy in their own `tests/integration/testdata/`
trees. `refs/scripts/check_rival_canonical_fixture.sh` enforces byte
identity and is wired into `just gap-check-rival-contract`.

### Roundtrip self-test

`tests/integration/rival_contract_roundtrip_test.go` reads the produced
fixture and runs sj's own `ParseRivalContractBody` +
`ParseRivalContractMetadata` against it (parity check against the same
parser the consumer-side roundtrip tests run). Companion fixtures
`legacy-spec.md` and `event-sourced-v1.md` exercise the legacy fallback
and the v1.1 `domain_style: event-sourced` path.

### Amendment cycle (sj-side)

`tests/integration/rival_amendment_extract_test.go` is a black-box
`package integration` test that consumes the SAME bytes amadeus emits
from its own white-box golden test. The shared committed fixture lives
at `tests/integration/testdata/rival/cross-tool/amadeus-emitted-correction.md`
(byte-identical copy of am's
`internal/session/testdata/rival/cross-tool/amadeus-emitted-correction.md`).
The test runs sj's real `ExtractContractAmendments` against the body and
then `ComposeAmendedSpecification` to assert:

- the new D-Mail's `metadata.contract_revision = previous + 1`,
- `metadata.supersedes = previous_dmail_name`,
- the `## Amendment Lineage` trailer cites the corrective D-Mail,
- the previous D-Mail on disk is unchanged (append-only invariant).

`refs/scripts/check_rival_amendment_fixture.sh` enforces byte-identity
of the am SoT and the sj copy and is wired into
`just gap-check-rival-contract`.

### Binary E2E (`rival export reasons`)

`tests/e2e/rival_export_e2e_test.go` (build tag `e2e`) compiles the
actual `sightjack` binary via `go build -o $TMPDIR/sightjack
./cmd/sightjack`, writes a sample D-Mail to a temp directory, and runs
the binary as a subprocess. Assertions cover:

- markdown stdout vs. committed golden,
- `--format json` mode,
- `--output <file>` mode (golden diff against on-disk file),
- exit code propagation through the cobra layer.

Default `go test ./...` skips this test (build tag); CI's `e2e` job runs
it via `go test -tags e2e ./tests/e2e/...`.

### Test inventory delta (sj only)

| Phase | Tests added (sj) |
|---|---|
| 1.2A — produce + roundtrip | 7 integration tests |
| 1.2B — amendment extract+compose | (covered above) |
| 1.2C — `rival export reasons` E2E | 4 e2e tests |

Total: 11 cross-component tests added in v1.2 for sightjack alone.
Cross-tool consumers (pt/am/dom) add their own roundtrip tests against
the sj-emitted fixture; see each tool's `docs/rival-contract-v1.md`
v1.2 section.

### What did NOT change

- Schema (still `rival-contract-v1`; v1 invariants 1-13 maintained).
- D-Mail v1 transport, kinds, routing, archive behavior.
- Producer code paths (`ComposeSpecification`, `ComposeAmendedSpecification`,
  `RenderRivalContract`).
- The `rival export reasons` subcommand (v1.1 surface).
- Any consumer-side parser or projection.

v1.2 is purely additive test code, fixtures, and gap-check guards.
