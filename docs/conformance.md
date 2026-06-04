# What / Why / How Conformance

This is the single source of truth for sightjack's purpose, design rationale, and implementation approach.
Referenced from [README.md](../README.md) and [docs/README.md](README.md).

| Aspect | Description |
|--------|-------------|
| **What** | MCP server + data plane for SIREN-inspired issue architecture: serves scan/wave read models and persists scan strictness |
| **Why** | Let a human-initiated claude-code session plan waves from durable local read models without the Go CLI owning inference |
| **How** | `sightjack mcp` serves MCP tools (`next_wave`, `get_scan_result`, `update_strictness`); the `/sightjack-scan` skill in the claude-code session owns issue scanning, wave planning, D-Mail composition, and any LLM/tool use |
| **Input** | `.siren/` config, event store, scan result files, MCP tool arguments |
| **Output** | MCP tool responses, atomically updated `.siren/config.yaml`, rebuilt projections for inspection commands |
| **Telemetry** | OTel spans on command roots and MCP tool handlers; `context_budget.*` attributes remain available for recorded claude-code session metadata |
| **External Systems** | Local filesystem, OTel exporter (Jaeger/Weave), claude-code session as MCP client |

## Layer Architecture

```
cmd              --> usecase, session, usecase/port, harness, platform, domain  (composition root)
usecase          --> usecase/port, harness, domain                              (output port only)
usecase/port     --> domain (+ stdlib)                                          (interface contracts)
session          --> eventsource, usecase/port, harness, platform, domain       (adapter impl)
harness          --> domain (+ stdlib)                                          (decision/validation/specification)
eventsource      --> domain                                                     (event persistence adapter)
platform         --> domain (+ stdlib)                                          (cross-cutting infra)
domain           --> (nothing internal, stdlib only)                            (pure types/logic)
```

### Harness Layer

`harness` is the decision, validation, and specification layer between the LLM and the environment. It contains three sub-packages arranged on the AutoHarness spectrum:

| Sub-package | Responsibility | May import |
|-------------|---------------|------------|
| `harness/policy` | Deterministic wave/scan/config logic (sorting, merging, filtering) | `domain` only |
| `harness/verifier` | Validation of wave results and provider error classification | `domain`, `harness/policy` |
| `harness/filter` | Prompt registry, rendering, building, and optimizer (LLM action space) | `domain`, `harness/policy`, `harness/verifier` |

The facade `harness/harness.go` re-exports all public symbols. External callers (cmd, usecase, session, eventsource) MUST import the facade, not the sub-packages directly. Sub-packages MUST NOT import the facade (cycle prevention).

Ref: `.semgrep/layers-harness.yaml`, `refs/opsx/semgrep-layer-contract.md`

### Event Source Layer

`eventsource` is the event persistence adapter based on the [AWS Event Sourcing pattern](https://docs.aws.amazon.com/prescriptive-guidance/latest/cloud-design-patterns/event-sourcing.html).
Its responsibility is limited to append, load, and replay of domain events.
Event store implementation MUST NOT exist outside `internal/eventsource`.
`session` uses `eventsource` as a client but does not implement event persistence itself.

Key constraints enforced by semgrep (ERROR severity):

- `usecase --> session` PROHIBITED (must use output port interfaces)
- `cmd --> eventsource` PROHIBITED (ADR S0008)
- `domain` has no I/O, no `context.Context`
- `domain --> harness` PROHIBITED
- `eventsource --> harness` PROHIBITED
- `harness/policy --> harness/verifier` or `harness/filter` PROHIBITED

Ref: `.semgrep/layers.yaml`, `.semgrep/layers-harness.yaml`, `refs/opsx/semgrep-layer-contract.md`, ADR S0007

## Domain Primitives & Parse-Don't-Validate

Domain command types use the Parse-Don't-Validate pattern:

- Domain primitives (`RepoPath`, `SessionID`, `ClusterName`, `Topic`, `Lang`) validate in `New*()` constructors — invalid values are rejected at parse time
- Command types use unexported fields with `New*Command()` constructors that accept only pre-validated primitives
- Commands are always-valid by construction — no `Validate() []error` methods exist
- Usecase layer receives always-valid commands with no validation boilerplate
- Semgrep rule `domain-no-validate-method` prevents reintroduction of `Validate() []error`

Ref: `.semgrep/layers.yaml`, ADR S0029

## MCP Pivot Boundary

Sightjack does not own model inference or run the retired scan / wave / discuss / apply pipeline from the Go CLI. LLM execution is owned by a human-initiated Claude Code session attached to `sightjack mcp`.

- `sightjack mcp` implements the MCP lifecycle (`initialize`, `notifications/initialized`, `tools/list`, `tools/call`) over stdio.
- `sightjack.next_wave` and `sightjack.get_scan_result` read durable scan/wave state for the session.
- `sightjack.update_strictness` is the only MCP tool that mutates state, and it atomically updates `.siren/config.yaml`.
- The `/sightjack-scan` skill composes D-Mails and performs LLM/tool-driven planning from the claude-code session.

Ref: ADR 0018, `internal/session/mcp_server.go`, `plugins/sightjack/skills/sightjack-scan/SKILL.md`

## Cross-Tool Conformance

All 4 tools (phonewave, sightjack, paintress, amadeus) maintain a What/Why/How conformance table in `docs/conformance.md` with the same structure. This prevents expression drift across README files.

## Harness Inventory (Track A)

| Sub-package | Key functions | Role |
|-------------|---------------|------|
| `harness/policy` | `ConfigPolicy`, `ConvergenceGate`, `ScanPolicy`, `WavePolicy`, `EvaluateExhaustion`, `RunGuard` | Deterministic decisions |
| `harness/verifier` | `ProviderError`, `WaveVerifier` | Validation rules |
| `harness/filter` | `PromptRegistry`, `PromptBuilder`, `PromptRender`, `Optimizer` | LLM action spaces |

Ref: ADR S0038, S0039

## Improvement Controller (Track D3/F)

The improvement controller resides in amadeus (ADR S0041). sightjack receives corrective D-Mails as a consumer but does not host improvement signal storage.
