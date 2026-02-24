# Event Sourcing Layer-First Refactoring Design

**Date:** 2026-02-24
**Status:** Approved
**Pattern Reference:** AWS Prescriptive Guidance — Event Sourcing
**Structure Reference:** gh CLI `internal/` package pattern

## Goal

Restructure the flat root `sightjack` package (~26 source files, ~20,000 LOC) into a layered `internal/` architecture following AWS Event Sourcing patterns and Go CLI best practices. No backward compatibility required.

## Architecture Overview

```
sightjack/                      # Public API: types + interfaces only
  internal/
    domain/                     # Pure functions (no I/O, no context.Context)
    eventsource/                # ES infrastructure (store, recorder, lifecycle)
    session/                    # Orchestration (CLI loop, Claude integration)
    cmd/                        # Cobra commands (existing, import path changes)
```

```
Legend / 凡例:
- sightjack (root): Public API types / 公開API型定義
- domain: Pure domain logic / 純粋ドメインロジック
- eventsource: Event Sourcing infrastructure / ES基盤
- session: Orchestration layer / オーケストレーション層
- cmd: CLI command definitions / CLIコマンド定義
```

## AWS Event Sourcing Pattern Mapping

| AWS Concept | sightjack Implementation | Package |
|---|---|---|
| Event Store | `FileEventStore` (JSONL append-only, per-event fsync) | `internal/eventsource` |
| Event | `Event` struct + 17 `EventType` constants | root `sightjack` |
| Aggregate | Wave logic (`CalcNewlyUnlocked`, `ApplyModifiedWave`, etc.) | `internal/domain` |
| Materialized View / Projection | `ProjectState(events) -> SessionState` | `internal/domain` |
| Command Handler | `RunSession` / `runInteractiveLoop` | `internal/session` |
| Snapshot | Not implemented (YAGNI) | - |

## Section 1: Root Package — Types + Interfaces Only

The root `sightjack` package becomes a pure type/interface definition layer. No functions, no logic, no I/O.

**Retained files (~4 files, ~700 LOC):**

| File | Contents |
|---|---|
| `types.go` | Core domain structs: `ScanResult`, `Cluster`, `Wave`, `WaveAction`, `WaveDelta`, `Ripple`, `DMail`, `ShibitoWarning`, `ADRConflict`, `ArchitectResponse`, `ScribeResponse`, approval enums, resume enums |
| `event.go` | `Event` struct, `EventType` constants (17), `EventSchemaVersion`, payload structs (14), `NewEvent()`, `ValidateEvent()`, `MarshalEvent()`, `UnmarshalEventPayload()` |
| `interfaces.go` | `EventStore`, `Recorder`, `Notifier`, `Approver`, `Handoff` interfaces |
| `config.go` | `Config`, `GateConfig`, `StrictnessLevel`, `StrictnessConfig`, `LabelsConfig` structs + `DefaultConfig()`, `LoadConfig()`, `ResolveStrictness()` |

**Design principle:** Any package can import `sightjack` (root) without pulling in I/O, network, or heavy dependencies. This breaks the current circular-import risk.

## Section 2: internal/domain — Pure Functions

Zero I/O. No `context.Context`. Signature pattern: `(input) -> (output, error)`.

**Files (~5 files, ~800 LOC):**

| File | Origin | Key Functions |
|---|---|---|
| `wave.go` | root `wave.go` | `CalcNewlyUnlocked`, `PartialApplyDelta`, `IsWaveApplyComplete`, `ApplyModifiedWave`, `PropagateWaveUpdate`, `BuildCompletedWaveMap`, `MergeCompletedStatus`, `RestoreWaves`, `BuildWaveStates`, `CheckCompletenessConsistency`, `CompletedWavesForCluster`, `mergeOldWaves` |
| `scan.go` | root `scanner.go` (pure parts) | `ClusterIssues`, `SanitizeName`, `CalcCompleteness`, scan result transformation helpers |
| `projection.go` | `internal/eventsource/projection.go` | `ProjectState(events) -> SessionState` (pure fold, no file I/O) |
| `config.go` | root `config.go` (pure parts) | `ResolveStrictness`, `DefaultConfig`, validation helpers |
| `prompt.go` | root `prompt.go` | Template rendering (`ResolveDoDSection`, template execution — pure string→string) |

**Test strategy:** Table-driven tests only. No mocks needed since everything is pure. Existing tests move with their functions.

## Section 3: internal/eventsource — ES Infrastructure

File I/O layer. Owns the `.siren/events/` directory structure.

**Files (~5 files, ~300 LOC):**

| File | Contents |
|---|---|
| `store_file.go` | `FileEventStore` — JSONL append-only, per-event fsync, `Append()`, `ReadAll()`, `ReadSince()`, `LastSequence()` |
| `recorder.go` | `SessionRecorder` — auto-sequencing, correlation/causation ID population |
| `lifecycle.go` | `ListExpiredEventFiles`, `PruneEventFiles` — retention policy |
| `path.go` | `EventsDir()`, `EventStorePath()` — path conventions |
| `loader.go` | `LoadState(store)`, `LoadLatestState(baseDir)` — find newest session, replay via `domain.ProjectState()` |

**Key change:** `projection.go` splits — pure fold logic goes to `internal/domain/projection.go`, file discovery/loading stays here as `loader.go`.

**Dependency:** `internal/eventsource` imports `sightjack` (root types) and `internal/domain` (for `ProjectState`).

## Section 4: internal/session — Orchestration

The "Command Handler" layer in AWS ES terms. Wires domain logic + ES infra + external services (Claude, Linear).

**Files (~16 files, ~2,500 LOC):**

| File | Origin | Responsibility |
|---|---|---|
| `session.go` | root `session.go` | `RunSession`, `RunResumeSession`, `RunRescanSession` |
| `interactive.go` | root `session.go` | `runInteractiveLoop`, `selectPhase`, `approvalPhase`, `applyPhase` |
| `claude.go` | root `claude.go` | `RunClaude`, `RunClaudeOnce`, tool allow-lists |
| `prompt.go` | root `cli.go` | `ScanLine`, `PromptWaveSelection`, `PromptWaveApproval`, etc. |
| `display.go` | root `navigator.go` | `RenderNavigator`, `RenderMatrixNavigator`, `Display*` functions |
| `scan.go` | root `scanner.go` | `RunScan`, `RunParallel` (I/O parts) |
| `wave_apply.go` | root `wave_apply.go` | Wave execution with Claude |
| `architect.go` | root `architect.go` | Architect agent integration |
| `scribe.go` | root `scribe.go` | Scribe agent + ADR generation |
| `wave_generator.go` | root `wave_generator.go` | Wave generation via Claude |
| `dmail.go` | root `dmail.go` | D-Mail parsing, inbox drain |
| `notify.go` | root `notify.go` | `LocalNotifier`, `CmdNotifier` |
| `approve.go` | root `approve.go` | `StdinApprover`, `CmdApprover`, `AutoApprover` |
| `gate.go` | root `gate.go` | Convergence gate logic |
| `doctor.go` | root `doctor.go` | Health check commands |
| `init_cmd.go` | root `init.go` | Project initialization |
| `archive.go` | root `archive.go` | Archive/maildir operations |
| `json_normalize.go` | root `json_normalize.go` | JSON normalization utility |

**DI pattern:** `session.New(cfg, store, recorder, notifier, approver)` — all dependencies injected. `Recorder` is the `sightjack.Recorder` interface, not the concrete `eventsource.SessionRecorder`.

## Section 5: internal/cmd — Cobra Commands

Existing `internal/cmd/` package. Changes are import path updates only.

**Responsibility:** DI wiring + Cobra command tree. Creates concrete instances and passes them to `internal/session`.

```go
// internal/cmd/run.go (conceptual)
store := eventsource.NewFileEventStore(path)
recorder := eventsource.NewSessionRecorder(store, sessionID)
notifier := session.BuildNotifier(cfg)
approver := session.BuildApprover(cfg, input, output)
return session.RunSession(ctx, cfg, store, recorder, notifier, approver)
```

## Section 6: Dependency Direction

```
cmd  -->  session  -->  domain
 |           |            ^
 |           v            |
 +----->  eventsource ----+
              |
              v
           sightjack (root: types only)
```

**Rules:**
- Root imports nothing from `internal/`
- `domain` imports only root (types)
- `eventsource` imports root + domain
- `session` imports root + domain + eventsource
- `cmd` imports everything

**Test strategy per layer:**

| Layer | Mock Policy | Focus |
|---|---|---|
| domain | Zero mocks (pure functions) | Table-driven, edge cases |
| eventsource | Real temp files (os.MkdirTemp) | File I/O correctness, fsync |
| session | Mock EventStore + Recorder interfaces | Orchestration flow |
| cmd (integration) | Minimal — real wiring, fake Claude | CLI flag parsing, DI |
| e2e | Zero mocks | Docker-based, real Claude binary |

## Section 7: Before / After Summary

| Metric | Before | After |
|---|---|---|
| Root package files | 26 | ~4 (types + interfaces) |
| Root package LOC | ~20,000 | ~700 |
| internal/ packages | 1 (cmd) | 4 (cmd, domain, eventsource, session) |
| Projection location | eventsource | domain (pure) + eventsource (loader) |
| Circular import risk | High (flat pkg) | Eliminated (layered deps) |
| Test isolation | Mixed | Pure / I/O / Integration clearly separated |

## Migration Strategy

- Move files in dependency order: domain (leaf) first, then eventsource, then session, then cmd
- Each commit: move file(s) + update imports + verify `go build ./...` + `go test ./...`
- Tidy First: structural changes only, zero behavioral changes
- No backward compatibility shims needed
