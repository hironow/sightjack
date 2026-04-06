# Testing Strategy

## Test Layers

| Layer | Directory | Build Tag | Dependencies | CI |
|-------|-----------|-----------|-------------|-----|
| Unit | `internal/*/` | none | none | always |
| Integration | `tests/integration/` | none | SQLite | always |
| Scenario | `tests/scenario/` | `scenario` | fake-claude, fake-gh, all 4 tool binaries | CI default (L1+L2) |
| E2E | `tests/e2e/` | `e2e` | Docker, real services | manual / nightly |

## Unit Tests

- Located in `internal/*/` alongside production code
- No build tags required
- Minimize mock usage; prefer real code
- Run: `go test ./internal/... -count=1`

## Integration Tests

- Located in `tests/integration/`
- Test component interactions with real SQLite
- Run: `go test ./tests/integration/... -count=1`

## Scenario Tests

- Located in `tests/scenario/`
- Build tag: `//go:build scenario`
- Requires all 4 sibling tool repos at the same parent directory
- TestMain builds all 4 binaries + fake-claude + fake-gh
- Override sibling paths with env vars: `PHONEWAVE_REPO`, `SIGHTJACK_REPO`, `PAINTRESS_REPO`, `AMADEUS_REPO`

### Test Levels

| Level | Focus | Timeout |
|-------|-------|---------|
| L1 | Single closed loop | 120s |
| L2 | Multi-issue scenarios | 180s |
| L3 | Concurrent operations | 300s |
| L4 | Fault injection, recovery | 600s |

Run: `just test-scenario` (L1+L2) or `just test-scenario-all`

### Scenario Test Observers

The `Observer` type (`tests/scenario/observer_test.go`) provides reusable assertion helpers for scenario tests. Observers wrap a `Workspace` and `testing.T` to verify post-run state without duplicating assertion logic across test functions.

| Observer Helper | Purpose |
|-----------------|---------|
| `AssertMailboxState` | Verify file counts in mailbox directories |
| `AssertAllOutboxEmpty` | Verify all tool outboxes are drained |
| `AssertArchiveContains` | Verify D-Mail kinds in archive directories |
| `AssertDMailKind` | Verify a specific D-Mail's kind field |
| `WaitForClosedLoop` | Poll for specification/report/feedback delivery |
| `AssertSirenConfigStrictness` | Verify estimated strictness in config |
| `AssertEventCount` / `AssertEventExists` | Verify event store contents |
| `AssertWaitingModeNotActive` | Document that scenario tests disable waiting mode |
| `AssertLabelsDisabled` | Document that scenario tests disable labels |
| `AssertScanWarningsExist` | Verify scan produced warnings |
| `AssertSessionResumed` / `AssertSessionRescanned` | Verify session lifecycle events |
| `AssertCompletenessUpdated` | Verify completeness tracking events |
| `AssertADRFileExists` / `AssertADRContainsSections` | Verify ADR generation |
| `AssertSpecificationFields` / `AssertReportFields` | Verify D-Mail field completeness |
| `AssertWaveApplyFailed` | Verify wave apply failure events |
| `AssertPromptContains` | Verify prompt log contents |
| `AssertConfigValue` | Verify config key-value pairs |
| `AssertArchitectNotCalled` | Document auto-approve architect bypass |

### Wave Lifecycle Guards

The scenario test infrastructure validates wave lifecycle integrity through guards wired into the session pipeline:

- **ValidateWavePrerequisites** — removes dangling prerequisites referencing waves not in the wave set (wired into `RunResumeSession`)
- **RepairLockedWaves** — unlocks waves whose prerequisites are all met but status is still "locked" (wired into `RunResumeSession`)
- **PruneStaleWaves** — removes waves whose cluster is no longer in the valid cluster set (wired into `RunRescanSession`)
- **ValidateWaveApplyResult** — rejects nil, empty, or over-counted apply results (wired into `RunWaveApply`)
- **FilterEmptyClassifications** — removes clusters with zero issue IDs from classification (wired into `RunScan`)

### Error Fingerprinting

The `ErrorFingerprint` algorithm (`internal/domain/error_fingerprint.go`) provides stable error identity for detecting repeated failures:

- `ErrorFingerprint(errMsg)` — SHA-256 based 16-character hash for stable error identity
- `ClassifyError(errMsg)` — classifies errors as `structural` (persistent, requires intervention) or `transient` (may self-resolve)
- `DetectRepeatedPattern(fingerprints, threshold)` — detects when the same error fingerprint appears at least `threshold` times
- `MarkStalled(waveID, clusterName, reason)` — transitions a wave to "stalled" status and emits a `wave.stalled` event

### Stall Escalation

When repeated structural errors are detected on a wave, a `stall-escalation` D-Mail is composed via `ComposeStallEscalation` (`internal/session/dmail_stall.go`). The D-Mail includes the wave key, escalation reason, and the list of structural errors. Severity is set to "high" with action "escalate".

## E2E Tests

- Located in `tests/e2e/`
- Build tag: `//go:build e2e`
- Docker compose based (`tests/e2e/compose-e2e.yaml`)
- All dependencies must be real — mocks are strictly prohibited
- Run: `just test-e2e` (requires Docker)

## Public API Test Policy

Unit tests prefer **external test packages** (`package xxx_test`) over white-box packages (`package xxx`). External tests exercise only the public API surface, which:

- Validates the API contract that external consumers depend on
- Catches accidental API breakage through compilation
- Permits internal refactoring without test changes
- Reduces coupling between tests and implementation details

White-box tests (`package xxx`) are reserved for cases that require access to unexported symbols (e.g., testing internal state machines, concurrency internals). Bridge constructors in `export_test.go` files expose specific unexported symbols for external tests when needed.

### CI Enforcement

The `package-audit` CI job enforces minimum external test ratios:

| Scope | Threshold |
|-------|-----------|
| `internal/` | >= 85% |
| `internal/session/` | >= 90% |

Run locally: `just test-package-audit`

### White-Box Test Rationale

Every same-package test file (`package xxx`, not `package xxx_test`) must include a `// white-box-reason:` comment immediately after the package declaration, explaining why public API testing is insufficient.

Format: `// white-box-reason: <concise reason referencing unexported symbols>`

The `package-audit` CI job and `just test-package-rationale-audit` enforce this requirement. New same-package test files without the comment will fail CI.

## Quality Command Contract

### Local Commands

| Command | Purpose | Dependencies |
|---------|---------|-------------|
| `just lint` | Full lint pass | vet, semgrep, root-guard, nosemgrep-audit, lint-md |
| `just check` | Pre-commit gate | fmt, vet, semgrep, root-guard, nosemgrep-audit, test, docs-check |
| `just semgrep` | Semgrep ERROR rules | semgrep |
| `just nosemgrep-audit` | Validate nosemgrep tags | grep/awk |
| `just semgrep-test` | Test semgrep rules against fixtures | semgrep |

### CI Jobs

| Job | Steps |
|-----|-------|
| `semgrep` | `just semgrep` + `just nosemgrep-audit` + `just semgrep-test` |
| `package-audit` | threshold check (inline) + `just test-package-rationale-audit` |
| `test` | build + vet + test + race |
| `docs-check` | docgen + dead links + vocabulary |

### Failure Workflow

1. `just lint` fails locally: fix the issue before committing.
2. `just nosemgrep-audit` fails: add `[permanent]` or `[expires: YYYY-MM-DD]` tag to the nosemgrep annotation.
3. `just semgrep` fails: fix the code or add a tagged nosemgrep annotation if false positive.

## Running Tests

```bash
# Unit + integration (default CI)
just test

# Scenario tests (L1+L2, CI default)
just test-scenario

# E2E (requires Docker)
just test-e2e

# All semgrep rules
just semgrep
just semgrep-test
just semgrep-warnings
```
