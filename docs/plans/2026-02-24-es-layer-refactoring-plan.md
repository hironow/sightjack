# ES Layer-First Refactoring Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Restructure flat root `sightjack` package (26 source files) into layered `internal/` architecture: root = types/interfaces only, `internal/domain` = pure functions, `internal/session` = orchestration, `internal/eventsource` = ES infra.

**Architecture:** Move files bottom-up to avoid import cycles. Extract interfaces to root first, then move implementations to `internal/session`, then extract pure functions to `internal/domain`. Each commit is [STRUCTURAL] — zero behavioral changes.

**Tech Stack:** Go 1.23+, cobra v1.10.2, OpenTelemetry, pond worker pool

**Design doc:** `docs/plans/2026-02-24-es-layer-refactoring-design.md`

---

## Migration Rules

Every task MUST:
1. Compile: `go build ./...`
2. Pass vet: `go vet ./...`
3. Pass tests: `go test ./... -count=1 -timeout 300s`
4. Result in a single `[STRUCTURAL]` commit
5. NOT change any behavior

## Dependency Order Rationale

Files form this dependency DAG (function calls between root files):

```
session.go
  |-- cli.go, navigator.go (UI)
  |-- gate.go --> dmail.go, notify.go, approve.go
  |-- scanner.go <--> wave.go (mutual: SanitizeName / NormalizeWavePrerequisites)
  |       |-- claude.go (RunClaudeOnce)
  |       |-- architect.go, scribe.go, wave_generator.go
  |-- doctor.go --> claude.go
  |-- prompt.go, state.go, init.go, handoff.go (leaves)
  +-- archive.go --> dmail.go (MailDir)
```

Cycle prevention rule: A root file CANNOT import `internal/session`. So when file B moves to `internal/session`, any root file A that calls B's functions would create a cycle (`root -> internal/session -> root`). Files must move AFTER all their root-package callers have moved.

Safe order: session.go first (only called from `internal/cmd`), then leaves, then clusters.

---

### Task 1: Extract interfaces to root interfaces.go

Extract all interfaces from their current files into a new `interfaces.go`. This lets implementations move to `internal/session` while callers keep referencing `sightjack.Notifier`, etc.

**Files:**
- Create: `interfaces.go`
- Modify: `notify.go` — remove `Notifier` interface definition (keep implementations)
- Modify: `approve.go` — remove `Approver` interface definition (keep implementations)
- Modify: `handoff.go` — remove `Handoff` interface + `HandoffResult` struct (keep `ReadyIssueIDs` function)
- Modify: `event.go` — remove `EventStore` interface definition (keep Event types)
- Modify: `recorder.go` — remove `Recorder` interface (keep NopRecorder, LoggingRecorder)

**Step 1: Create interfaces.go**

```go
package sightjack

import "context"

// EventStore persists and retrieves domain events.
type EventStore interface {
    Append(events ...Event) error
    ReadAll() ([]Event, error)
    ReadSince(afterSeq int64) ([]Event, error)
    LastSequence() (int64, error)
}

// Recorder records domain events during a session.
type Recorder interface {
    Record(eventType EventType, payload any) error
}

// Notifier delivers notifications to the user.
type Notifier interface {
    Notify(ctx context.Context, title, message string) error
}

// Approver requests user approval for convergence gates.
type Approver interface {
    RequestApproval(ctx context.Context, message string) (bool, error)
}

// Handoff defines the integration contract for downstream execution agents.
type Handoff interface {
    HandoffReady(ctx context.Context, issueIDs []string) error
    ReportIssue(ctx context.Context, issueID string, finding string) error
}

// HandoffResult tracks the outcome of a handoff for a single issue.
type HandoffResult struct {
    IssueID string
    Status  string
    Error   string
}
```

**Step 2: Remove interface definitions from source files**

In each source file, delete ONLY the interface/type that was copied to interfaces.go. Keep all function implementations and other types.

- `notify.go`: delete `Notifier` interface (lines ~6-8)
- `approve.go`: delete `Approver` interface
- `handoff.go`: delete `Handoff` interface + `HandoffResult` struct (keep `ReadyIssueIDs` function)
- `event.go`: delete `EventStore` interface
- `recorder.go`: delete `Recorder` interface (keep NopRecorder, LoggingRecorder)

**Step 3: Verify**

```bash
go build ./... && go vet ./... && go test ./... -count=1 -timeout 300s
```

**Step 4: Commit**

```bash
git add interfaces.go notify.go approve.go handoff.go event.go recorder.go
git commit -m "refactor: extract interfaces to interfaces.go [STRUCTURAL]"
```

---

### Task 2: Move session.go → internal/session/

session.go (848 lines) is the orchestration core. Called ONLY from `internal/cmd/run.go`. All other calls go OUT from session.go to other root files.

**Files:**
- Create: `internal/session/` directory
- Move: `session.go` → `internal/session/session.go`
- Move: `session_test.go` → `internal/session/session_test.go`
- Move: `export_test.go` → `internal/session/export_test.go`
- Move: `lifecycle_test.go` → `internal/session/lifecycle_test.go`
- Modify: `internal/cmd/run.go` — change imports

**Step 1: Create directory**

```bash
mkdir -p internal/session
```

**Step 2: Move and update session.go**

```bash
git mv session.go internal/session/session.go
```

Edit `internal/session/session.go`:
- Change `package sightjack` → `package session`
- Add import: `sightjack "github.com/hironow/sightjack"`
- Qualify ALL root type references: `*Config` → `*sightjack.Config`, `Wave` → `sightjack.Wave`, `ScanResult` → `*sightjack.ScanResult`, etc.
- Qualify ALL root function calls: `RunScan(...)` → `sightjack.RunScan(...)`, `CalcNewlyUnlocked(...)` → `sightjack.CalcNewlyUnlocked(...)`, etc.
- Qualify ALL root constant/var references: `EventSessionStarted` → `sightjack.EventSessionStarted`, `StrictnessAlert` → `sightjack.StrictnessAlert`, etc.

Use the compiler to find every unresolved reference:
```bash
go build ./internal/session/ 2>&1 | head -50
```

**Step 3: Move test files**

```bash
git mv session_test.go internal/session/session_test.go
git mv export_test.go internal/session/export_test.go
git mv lifecycle_test.go internal/session/lifecycle_test.go
```

- Change `package sightjack_test` → `package session_test` in test files
- Change `package sightjack` → `package session` in export_test.go
- Add `sightjack "github.com/hironow/sightjack"` import
- Update all type/function references to use `sightjack.` prefix or drop it (for things now in session package)

**Step 4: Update internal/cmd/run.go**

```go
import (
    // existing imports...
    "github.com/hironow/sightjack/internal/session"
)
```

Change calls:
- `sightjack.RunSession(...)` → `session.RunSession(...)`
- `sightjack.RunResumeSession(...)` → `session.RunResumeSession(...)`
- `sightjack.RunRescanSession(...)` → `session.RunRescanSession(...)`
- `sightjack.CanResume(...)` → `session.CanResume(...)`
- `sightjack.ResumeSession(...)` → `session.ResumeSession(...)`
- `sightjack.ResumeScanDir(...)` → `session.ResumeScanDir(...)`

**Step 5: Verify and commit**

```bash
go build ./... && go vet ./... && go test ./... -count=1 -timeout 300s
git add -A && git commit -m "refactor: move session.go to internal/session [STRUCTURAL]"
```

---

### Task 3: Move CLI UI files → internal/session/

cli.go (prompts) and navigator.go (display rendering). Called from session.go (already in internal/session) and several internal/cmd files.

**Files:**
- Move: `cli.go` → `internal/session/cli.go`
- Move: `navigator.go` → `internal/session/navigator.go`
- Move: `cli_test.go` → `internal/session/cli_test.go`
- Move: `navigator_test.go` → `internal/session/navigator_test.go`
- Modify: `internal/session/session.go` — drop `sightjack.` prefix for CLI/navigator calls
- Modify: `internal/cmd/select.go`, `internal/cmd/show.go` — add session import

**Step 1: Move files**

```bash
git mv cli.go internal/session/cli.go
git mv navigator.go internal/session/navigator.go
git mv cli_test.go internal/session/cli_test.go
git mv navigator_test.go internal/session/navigator_test.go
```

**Step 2: Update package declarations and imports**

In moved files: `package sightjack` → `package session`, add `sightjack` import, qualify root types.

In `internal/session/session.go`: remove `sightjack.` prefix for functions now in same package (e.g., `sightjack.PromptWaveSelection` → `PromptWaveSelection`, `sightjack.RenderMatrixNavigator` → `RenderMatrixNavigator`).

**Step 3: Update internal/cmd callers**

Files in `internal/cmd/` that call CLI/navigator functions (select.go, show.go, etc.) need:
```go
import "github.com/hironow/sightjack/internal/session"
```
And change `sightjack.PromptWaveSelection` → `session.PromptWaveSelection`, etc.

**Step 4: Verify and commit**

```bash
go build ./... && go vet ./... && go test ./... -count=1 -timeout 300s
git add -A && git commit -m "refactor: move cli.go, navigator.go to internal/session [STRUCTURAL]"
```

---

### Task 4: Move claude.go, dmail.go, gate.go, archive.go, prompt.go → internal/session/

These must move together because:
- gate.go calls `DrainInboxFeedback` from dmail.go
- archive.go calls `MailDir`, `ArchiveDir` from dmail.go
- All are called from session.go (already moved) or internal/cmd

claude.go and prompt.go are leaves (no root callers after session.go moved).

**Files to move (5 source + 5 test):**
- `claude.go` → `internal/session/claude.go`
- `dmail.go` → `internal/session/dmail.go`
- `gate.go` → `internal/session/gate.go`
- `archive.go` → `internal/session/archive.go`
- `prompt.go` → `internal/session/prompt.go`
- Corresponding `*_test.go` files

**Step 1: Move all files**

```bash
git mv claude.go internal/session/claude.go
git mv dmail.go internal/session/dmail.go
git mv gate.go internal/session/gate.go
git mv archive.go internal/session/archive.go
git mv prompt.go internal/session/prompt.go
git mv claude_test.go internal/session/claude_test.go
git mv dmail_test.go internal/session/dmail_test.go
git mv gate_test.go internal/session/gate_test.go
git mv archive_test.go internal/session/archive_test.go
git mv prompt_test.go internal/session/prompt_test.go
```

**Step 2: Update package + imports in moved files**

Each file: `package sightjack` → `package session`, add sightjack import, qualify root types.

claude.go also has `var newCmd = exec.Command` — this is the unexported test hook. export_test.go (already in internal/session) exposes it via `SetNewCmd`. Verify this still works.

gate.go: calls to `FilterConvergence`, `DrainInboxFeedback` become unqualified (same package).

**Step 3: Update session.go and internal/cmd**

In `internal/session/session.go`: drop `sightjack.` prefix for newly same-package functions:
- `sightjack.RunClaudeOnce` → `RunClaudeOnce`
- `sightjack.MonitorInbox` → `MonitorInbox`
- `sightjack.RunConvergenceGateWithRedrain` → `RunConvergenceGateWithRedrain`
- `sightjack.buildNotifier` → `buildNotifier`
- `sightjack.buildApprover` → `buildApprover`
- etc.

In internal/cmd files: add session import where needed (e.g., `internal/cmd/archive_prune.go` for `session.PruneArchive`).

**Step 4: Create session-local OTel tracer**

claude.go and gate.go use the root-level `tracer` var. Since they've moved, add to `internal/session/`:

```go
// At top of claude.go or a new telemetry.go in internal/session/
var tracer = otel.Tracer("session")
```

This replaces the root `tracer` reference. Each package having its own named tracer is the OTel convention.

**Step 5: Verify and commit**

```bash
go build ./... && go vet ./... && go test ./... -count=1 -timeout 300s
git add -A && git commit -m "refactor: move claude, dmail, gate, archive, prompt to internal/session [STRUCTURAL]"
```

---

### Task 5: Move notify.go, approve.go, state.go, init.go, json_normalize.go, handoff.go → internal/session/

All leaf files with no root-package callers (after previous moves). Interfaces already extracted to root `interfaces.go`.

**Files to move (6 source + 6 test):**
- `notify.go` → `internal/session/notify.go`
- `approve.go` → `internal/session/approve.go`
- `state.go` → `internal/session/state.go`
- `init.go` → `internal/session/init_cmd.go` (rename to avoid collision with Go's init())
- `json_normalize.go` → `internal/session/json_normalize.go`
- `handoff.go` → `internal/session/handoff.go`
- Corresponding `*_test.go` files (init_test.go → init_cmd_test.go)

**Step 1: Move files**

```bash
git mv notify.go internal/session/notify.go
git mv approve.go internal/session/approve.go
git mv state.go internal/session/state.go
git mv init.go internal/session/init_cmd.go
git mv json_normalize.go internal/session/json_normalize.go
git mv handoff.go internal/session/handoff.go
git mv notify_test.go internal/session/notify_test.go
git mv approve_test.go internal/session/approve_test.go
git mv state_test.go internal/session/state_test.go
git mv init_test.go internal/session/init_cmd_test.go
git mv json_normalize_test.go internal/session/json_normalize_test.go
git mv handoff_test.go internal/session/handoff_test.go
```

**Step 2: Update packages, imports, and callers**

Same pattern as previous tasks. session.go drops `sightjack.` prefix for these functions.

For internal/cmd callers:
- `internal/cmd/init.go`: `sightjack.RenderInitConfig` → `session.RenderInitConfig`
- `internal/cmd/init.go`: `sightjack.WriteGitIgnore` → `session.WriteGitIgnore`
- `internal/cmd/init.go`: `sightjack.InstallSkills` → `session.InstallSkills`
- `internal/cmd/init.go`: `sightjack.EnsureMailDirs` → `session.EnsureMailDirs`
- etc.

**Step 3: Verify and commit**

```bash
go build ./... && go vet ./... && go test ./... -count=1 -timeout 300s
git add -A && git commit -m "refactor: move notify, approve, state, init, json_normalize, handoff to internal/session [STRUCTURAL]"
```

---

### Task 6: Move doctor.go → internal/session/

doctor.go calls `RunClaudeOnce` (now in internal/session) and `LoadConfig` (stays in root). Since `RunClaudeOnce` already moved, doctor.go can now join internal/session.

**Files:**
- Move: `doctor.go` → `internal/session/doctor.go`
- Move: `doctor_test.go` → `internal/session/doctor_test.go`

**Step 1: Move and update**

```bash
git mv doctor.go internal/session/doctor.go
git mv doctor_test.go internal/session/doctor_test.go
```

Update package, imports. `RunClaudeOnce` becomes unqualified (same package). `LoadConfig` stays as `sightjack.LoadConfig`.

**Step 2: Update internal/cmd/doctor.go**

```go
import "github.com/hironow/sightjack/internal/session"
// sightjack.RunDoctor → session.RunDoctor
// sightjack.CheckConfig → session.CheckConfig
```

**Step 3: Verify and commit**

```bash
go build ./... && go vet ./... && go test ./... -count=1 -timeout 300s
git add -A && git commit -m "refactor: move doctor.go to internal/session [STRUCTURAL]"
```

---

### Task 7: Move scanner.go, wave.go, architect.go, scribe.go, wave_generator.go → internal/session/

The big interconnected cluster. These files have mutual dependencies:
- scanner.go ↔ wave.go (SanitizeName / NormalizeWavePrerequisites)
- architect.go, scribe.go, wave_generator.go → scanner.go (SanitizeName), wave.go, claude.go (already moved)

All must move in one commit to avoid import cycles.

**Files to move (5 source + 6 test):**
- `scanner.go` → `internal/session/scanner.go`
- `wave.go` → `internal/session/wave.go`
- `architect.go` → `internal/session/architect.go`
- `scribe.go` → `internal/session/scribe.go`
- `wave_generator.go` → `internal/session/wave_generator.go`
- `scanner_test.go` → `internal/session/scanner_test.go`
- `scanner_parallel_test.go` → `internal/session/scanner_parallel_test.go`
- `wave_test.go` → `internal/session/wave_test.go`
- `architect_test.go` → `internal/session/architect_test.go`
- `scribe_test.go` → `internal/session/scribe_test.go`
- `wave_generator_test.go` → `internal/session/wave_generator_test.go`

**Step 1: Move all files**

```bash
git mv scanner.go internal/session/scanner.go
git mv wave.go internal/session/wave.go
git mv architect.go internal/session/architect.go
git mv scribe.go internal/session/scribe.go
git mv wave_generator.go internal/session/wave_generator.go
git mv scanner_test.go internal/session/scanner_test.go
git mv scanner_parallel_test.go internal/session/scanner_parallel_test.go
git mv wave_test.go internal/session/wave_test.go
git mv architect_test.go internal/session/architect_test.go
git mv scribe_test.go internal/session/scribe_test.go
git mv wave_generator_test.go internal/session/wave_generator_test.go
```

**Step 2: Update packages and imports**

All files: `package sightjack` → `package session`.

Cross-calls within the cluster become unqualified (same package):
- `SanitizeName(...)` (was `sightjack.SanitizeName` or implicit)
- `NormalizeWavePrerequisites(...)`, `MergeWaveResults(...)`
- `RunClaudeOnce(...)` (already moved in Task 4)
- `WaveKey(...)`, `WaveApplyFileName(...)`

Root type references: qualify with `sightjack.` prefix.

In `internal/session/session.go`: drop `sightjack.` prefix for all newly same-package functions (RunScan, CalcNewlyUnlocked, PartialApplyDelta, IsWaveApplyComplete, ApplyModifiedWave, PropagateWaveUpdate, BuildCompletedWaveMap, MergeCompletedStatus, RestoreWaves, BuildWaveStates, CheckCompletenessConsistency, CompletedWavesForCluster, etc.)

**Step 3: Update internal/cmd callers**

Key files:
- `internal/cmd/scan.go`: `sightjack.RunScan` → `session.RunScan`
- `internal/cmd/waves.go`: `sightjack.RunWaveGenerate` → `session.RunWaveGenerate`
- `internal/cmd/apply.go`: `sightjack.RunWaveApply`, `sightjack.ToApplyResult` → session equivalents
- `internal/cmd/discuss.go`: `sightjack.RunArchitectDiscuss` → `session.RunArchitectDiscuss`
- `internal/cmd/adr.go`: `sightjack.NextADRNumber`, `sightjack.RenderADRFromDiscuss` → session equivalents
- `internal/cmd/nextgen.go`: `sightjack.RestoreWaves`, `sightjack.CompletedWavesForCluster`, `sightjack.GenerateNextWaves` → session equivalents
- `internal/cmd/show.go`: `sightjack.RestoreWaves`, `sightjack.RenderNavigator` → session equivalents

**Step 4: Verify and commit**

```bash
go build ./... && go vet ./... && go test ./... -count=1 -timeout 300s
git add -A && git commit -m "refactor: move scanner, wave, architect, scribe, wave_generator to internal/session [STRUCTURAL]"
```

---

### Task 8: Clean root to types-only

After Tasks 1-7, root should have only:
- `types.go` (currently model.go) — domain structs
- `event.go` — Event type + marshaling functions
- `interfaces.go` — all interfaces
- `config.go` — Config types + LoadConfig
- `logger.go` — Logger type + methods
- `telemetry.go` — OTel init + root span helpers
- `recorder.go` — NopRecorder + LoggingRecorder implementations

Plus test files: `model_test.go`, `event_test.go`, `config_test.go`, `logger_test.go`, `telemetry_test.go`, `recorder_test.go`

**Step 1: Rename model.go → types.go**

```bash
git mv model.go types.go
git mv model_test.go types_test.go
```

Update any references in other files (grep for `model.go` in comments if needed).

**Step 2: Merge recorder.go into interfaces.go**

Move NopRecorder and LoggingRecorder from recorder.go to interfaces.go (they implement the Recorder interface defined there). Delete recorder.go.

```bash
# After editing interfaces.go to include NopRecorder + LoggingRecorder:
git rm recorder.go
git mv recorder_test.go interfaces_test.go  # or merge into existing test
```

**Step 3: Verify root contents**

Root should now have exactly these source files:
```
types.go        (domain structs, ~341 LOC)
event.go        (Event + payloads + marshal, ~180 LOC)
interfaces.go   (all interfaces + NopRecorder + LoggingRecorder, ~100 LOC)
config.go       (Config types + LoadConfig, ~213 LOC)
logger.go       (Logger type + methods, ~74 LOC)
telemetry.go    (OTel init, ~71 LOC)
```

**Step 4: Verify and commit**

```bash
go build ./... && go vet ./... && go test ./... -count=1 -timeout 300s
git add -A && git commit -m "refactor: clean root to types-only, rename model.go to types.go [STRUCTURAL]"
```

---

### Task 9: Create internal/domain — extract pure functions

Move pure functions (no I/O, no context.Context, no side effects) from `internal/session` to `internal/domain`. These functions have signature pattern `(input) → (output, error)`.

**Files to create:**

| File | Source | Functions |
|------|--------|-----------|
| `internal/domain/wave.go` | `internal/session/wave.go` | `CalcNewlyUnlocked`, `PartialApplyDelta`, `IsWaveApplyComplete`, `ApplyModifiedWave`, `PropagateWaveUpdate`, `BuildCompletedWaveMap`, `MergeCompletedStatus`, `RestoreWaves`, `BuildWaveStates`, `CheckCompletenessConsistency`, `CompletedWavesForCluster`, `mergeOldWaves`, `WaveKey`, `NormalizeWavePrerequisites`, `MergeWaveResults`, `AvailableWaves`, `EvaluateUnlocks` |
| `internal/domain/scan.go` | `internal/session/scanner.go` | `SanitizeName`, `MergeScanResults`, `DetectFailedClusterNames`, `ChunkSlice`, `MergeClusterChunks` |
| `internal/domain/config.go` | root `config.go` | `ResolveStrictness`, `DefaultConfig`, `ValidLang` |
| `internal/domain/prompt.go` | `internal/session/prompt.go` | All `Render*Prompt` functions, `MatchDoDTemplate`, `ResolveDoDSection`, `FormatDoDSection` |
| `internal/domain/projection.go` | `internal/eventsource/projection.go` | `ProjectState` (the pure fold function, no file I/O) |

**Step 1: Create internal/domain/ with wave.go**

```bash
mkdir -p internal/domain
```

Create `internal/domain/wave.go` with pure wave functions. Import `sightjack "github.com/hironow/sightjack"` for types.

**Step 2: Update internal/session callers**

In `internal/session/wave.go`, `internal/session/session.go`, etc.:
```go
import "github.com/hironow/sightjack/internal/domain"
```
Change: `CalcNewlyUnlocked(...)` → `domain.CalcNewlyUnlocked(...)`, etc.

Remove the moved functions from `internal/session/wave.go`.

**Step 3: Create scan.go, config.go, prompt.go, projection.go**

Same pattern: create file in domain, update callers in session/eventsource, remove from source.

For `domain/config.go`: move `ResolveStrictness`, `DefaultConfig`, `ValidLang` from ROOT `config.go`. Root config.go keeps only type definitions + `LoadConfig` (I/O function).

For `domain/projection.go`: move `ProjectState` from `internal/eventsource/projection.go`. Keep `LoadState`, `LoadLatestState` in eventsource (they do file I/O).

**Step 4: Move tests**

Move corresponding test functions from session test files to `internal/domain/*_test.go`.

For `domain/projection.go`: split `internal/eventsource/projection_test.go` — pure ProjectState tests go to `internal/domain/projection_test.go`, LoadState/LoadLatestState tests stay.

**Step 5: Update internal/cmd callers**

`internal/cmd/nextgen.go`, `internal/cmd/show.go` that call `RestoreWaves`, `CompletedWavesForCluster`:
```go
import "github.com/hironow/sightjack/internal/domain"
// session.RestoreWaves → domain.RestoreWaves
```

**Step 6: Verify and commit**

```bash
go build ./... && go vet ./... && go test ./... -count=1 -timeout 300s
git add -A && git commit -m "refactor: create internal/domain with pure functions [STRUCTURAL]"
```

---

### Task 10: Restructure internal/eventsource

Split `projection.go` now that `ProjectState` moved to domain.

**Files:**
- Modify: `internal/eventsource/projection.go` — remove `ProjectState`, rename to `loader.go`
- Create: `internal/eventsource/loader.go` (if renaming is cleaner than editing)
- Update imports to use `domain.ProjectState`

**Step 1: Update projection.go**

Remove `ProjectState` function (moved to domain in Task 9). Rename file to `loader.go` since it now only contains `LoadState` and `LoadLatestState` (file I/O loading functions).

```bash
git mv internal/eventsource/projection.go internal/eventsource/loader.go
```

Add import:
```go
import "github.com/hironow/sightjack/internal/domain"
```

Change: `ProjectState(events)` → `domain.ProjectState(events)` in LoadState.

**Step 2: Update test file**

```bash
git mv internal/eventsource/projection_test.go internal/eventsource/loader_test.go
```

**Step 3: Verify and commit**

```bash
go build ./... && go vet ./... && go test ./... -count=1 -timeout 300s
git add -A && git commit -m "refactor: rename eventsource/projection.go to loader.go, use domain.ProjectState [STRUCTURAL]"
```

---

### Task 11: Final verification + ADR

**Step 1: Run full test suite with race detector**

```bash
go test ./... -race -count=1 -timeout 300s
```

**Step 2: Verify dependency direction**

```bash
# Root should NOT import any internal/ package
grep -r '"github.com/hironow/sightjack/internal' *.go || echo "OK: root has no internal imports"

# domain should only import root
grep -r '"github.com/hironow/sightjack/internal' internal/domain/*.go || echo "OK: domain has no internal imports"

# eventsource should import root + domain only
grep -r 'internal/session' internal/eventsource/*.go || echo "OK: eventsource does not import session"

# session can import everything
# cmd can import everything
```

**Step 3: Create ADR 0011**

Create `docs/adr/0011-internal-package-layer-extraction.md`:

```markdown
# 0011. Internal Package Layer Extraction

**Date:** 2026-02-24
**Status:** Accepted

## Context

The root `sightjack` package contained 26 source files mixing type definitions,
pure business logic, I/O operations, and CLI orchestration. This made it difficult
to test pure logic in isolation and created circular import risks when attempting
to extract packages.

Following AWS Event Sourcing patterns and gh CLI conventions for internal/ structure.

## Decision

Extract code from root into three internal packages:

1. **internal/domain** — pure functions (no I/O, no context.Context)
2. **internal/eventsource** — ES infrastructure (existing, now imports domain)
3. **internal/session** — orchestration (CLI, Claude, scanning, wave execution)

Root package retains only type definitions, interfaces, and minimal infrastructure
(Logger, Config loading, OTel init).

## Consequences

### Positive
- Pure domain logic testable without mocks (table-driven tests only)
- Clear dependency direction: cmd → session → domain, eventsource → domain
- No circular import risk
- Each layer has distinct test strategy

### Negative
- More packages to navigate
- Import paths longer in internal/cmd

### Neutral
- No API changes for external consumers (types stay at same import path)
- Test coverage unchanged
```

**Step 4: Commit**

```bash
git add docs/adr/0011-internal-package-layer-extraction.md
git commit -m "docs: add ADR 0011 for internal package layer extraction"
```

---

## Task Dependency Graph

```
Task 1 (interfaces.go)
  |
Task 2 (session.go)
  |
Task 3 (cli, navigator)
  |
Task 4 (claude, dmail, gate, archive, prompt)
  |
Task 5 (notify, approve, state, init, json_normalize, handoff)
  |
Task 6 (doctor)
  |
Task 7 (scanner, wave, architect, scribe, wave_generator)
  |
Task 8 (clean root)
  |
Task 9 (create internal/domain)
  |
Task 10 (restructure eventsource)
  |
Task 11 (ADR + verification)
```

All tasks are SEQUENTIAL. Each depends on the previous.

---

## Before / After

| Metric | Before | After |
|--------|--------|-------|
| Root source files | 26 | 6 (types, event, interfaces, config, logger, telemetry) |
| Root LOC | ~5,700 | ~980 |
| internal/ packages | 2 (cmd, eventsource) | 4 (cmd, domain, eventsource, session) |
| internal/session files | 0 | ~20 |
| internal/domain files | 0 | ~5 |
| Circular import risk | High | Eliminated |
| Pure function isolation | None | internal/domain (zero I/O) |

---

## Quick Reference: File Move Map

| Root File | Destination | Task |
|-----------|-------------|------|
| model.go | types.go (rename) | 8 |
| event.go | stays in root | 1 (interface extracted) |
| config.go | stays in root (pure funcs → domain T9) | 9 |
| logger.go | stays in root | — |
| telemetry.go | stays in root | — |
| recorder.go | merged into interfaces.go | 8 |
| interfaces.go | NEW in root | 1 |
| session.go | internal/session/ | 2 |
| cli.go | internal/session/ | 3 |
| navigator.go | internal/session/ | 3 |
| claude.go | internal/session/ | 4 |
| dmail.go | internal/session/ | 4 |
| gate.go | internal/session/ | 4 |
| archive.go | internal/session/ | 4 |
| prompt.go | internal/session/ | 4 |
| notify.go | internal/session/ | 5 |
| approve.go | internal/session/ | 5 |
| state.go | internal/session/ | 5 |
| init.go | internal/session/init_cmd.go | 5 |
| json_normalize.go | internal/session/ | 5 |
| handoff.go | internal/session/ | 5 |
| doctor.go | internal/session/ | 6 |
| scanner.go | internal/session/ | 7 |
| wave.go | internal/session/ | 7 |
| architect.go | internal/session/ | 7 |
| scribe.go | internal/session/ | 7 |
| wave_generator.go | internal/session/ | 7 |
