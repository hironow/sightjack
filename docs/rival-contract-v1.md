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
