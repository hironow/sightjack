# Policy Engine

PolicyEngine dispatches domain events to registered handlers (best-effort, fire-and-forget).
Errors are logged (if logger is non-nil) but never propagated — `Dispatch()` always returns nil.

## Location

- Engine: `internal/usecase/policy.go` (implements `port.EventDispatcher`)
- Policy declarations: `internal/domain/event.go` → `var Policies` (declarative WHEN/THEN registry)
- Wiring: `internal/usecase/emitter.go` → `sessionEventEmitter` (EventStore persistence + dispatch)

## Post jun15 MCP pivot: declarative registry only

The headless pipeline that executed these policies was retired with the jun15 MCP
pivot (ADR 0018). **No handlers are registered in production code today** — the
`domain.Policies` registry documents the reactive intent, and the reactions
themselves are driven by the human-initiated Claude Code session via the
`/sightjack-scan` skill and the sightjack MCP tools.

| Policy Name | WHEN [EVENT] | THEN [COMMAND] | Executed by (post-pivot) |
|---|---|---|---|
| WaveAppliedComposeReport | wave.applied | ComposeReport | Claude Code session (skill workflow) |
| ReportSentDeliverToPhonewave | report.sent | DeliverViaPhonewave | phonewave daemon (outbox watch) |
| ScanCompletedGenerateWaves | scan.completed | GenerateWaves | Claude Code session (skill workflow) |
| WaveCompletedNextGen | wave.completed | GenerateNextWaves | Claude Code session (skill workflow) |
| SpecificationSentDeliverToPhonewave | specification.sent | DeliverViaPhonewave | phonewave daemon (outbox watch) |

## Event Payload Format

| Event | Payload Type | Fields |
|---|---|---|
| scan.completed | `domain.ScanCompletedPayload` | `Completeness`, `Clusters`, `ShibitoCount` |
| wave.applied | (none) | uses `event.Type` |
| report.sent | (none) | uses `event.Type` |
| wave.completed | (none) | uses `event.Type` |
| specification.sent | (none) | uses `event.Type` |

## Dispatch Guarantee

Best-effort (at-most-once). Handler failures are silently logged.
No retry, no dead-letter queue, no error propagation to callers.
