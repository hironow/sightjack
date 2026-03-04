# Policy Engine

PolicyEngine dispatches domain events to registered handlers (best-effort, fire-and-forget).
Errors are logged (if logger is non-nil) but never propagated — `Dispatch()` always returns nil.

## Location

- Engine: `internal/usecase/policy.go`
- Handlers: `internal/usecase/policy_handlers.go`
- Policy definitions: `internal/domain/policy.go`
- Registration: `internal/usecase/session.go` → `registerSessionPolicies()`

## Event → Handler Mapping

| Policy Name | WHEN [EVENT] | THEN [COMMAND] | Side Effects |
|---|---|---|---|
| WaveAppliedComposeReport | wave.applied | ComposeReport | Log (Debug) |
| ReportSentDeliverToPhonewave | report.sent | DeliverViaPhonewave | Log (Debug) |
| ScanCompletedGenerateWaves | scan.completed | GenerateWaves | Log (Info) + Desktop notification (5s timeout) |
| WaveCompletedNextGen | wave.completed | GenerateNextWaves | Log (Debug) |
| SpecificationSentDeliverToPhonewave | specification.sent | DeliverViaPhonewave | Log (Debug) |

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

## Skeleton Handlers

WaveAppliedComposeReport, ReportSentDeliverToPhonewave, WaveCompletedNextGen,
and SpecificationSentDeliverToPhonewave are logging-only placeholders.
