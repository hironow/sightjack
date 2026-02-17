# Sightjack v0.5 Design: Session Resume

**Date:** 2026-02-18
**Scope:** Session Persistence (Resume only). Strictness levels, Shibito resurrection, and ADR contradiction detection deferred to v0.6+.
**Approach:** State-First Resume (Approach A)

## Decisions

- **Scope**: Session Resume only. Ship small.
- **Re-scan**: Hybrid — resume from saved state by default, explicit re-scan option available.
- **Resume UX**: Auto-detect existing state on `sightjack session`, prompt user with r/n/s choice.
- **State storage**: `.siren/state.json` only (no Linear labels, no git auto-commit).

## Section 1: Session State Expansion

### Current State

`SessionState` saves completion status, clusters, wave summaries, and ADR count. Missing: full wave data for resume, cached scan result path.

### Changes

**WaveState expansion** — add full Actions, Description, Delta so waves can be reconstructed without re-scanning:

```go
type WaveState struct {
    ID            string       `json:"id"`
    ClusterName   string       `json:"cluster_name"`
    Title         string       `json:"title"`
    Status        string       `json:"status"`
    Prerequisites []string     `json:"prerequisites,omitempty"`
    ActionCount   int          `json:"action_count"`
    Actions       []WaveAction `json:"actions,omitempty"`      // NEW
    Description   string       `json:"description,omitempty"`  // NEW
    Delta         WaveDelta    `json:"delta,omitempty"`        // NEW
}
```

**ScanResult caching** — after scan completes, serialize `ScanResult` to `scanDir/scan_result.json`. On resume, load from this file instead of re-scanning Linear.

**SessionState addition:**

```go
ScanResultPath string `json:"scan_result_path,omitempty"` // NEW
```

### State Save Timing

- Current: end of session only.
- v0.5: **after each wave completion** + end of session. Prevents data loss on mid-session interruption.

## Section 2: Resume Flow

```
sightjack session
    |
    ReadState(baseDir)
    |
    +--- state exists ---> PromptResume()
    |                        |
    |                  +-----+------+-----+
    |                  | r         | n         | s
    |                  |           |           |
    |              ResumeSession  RunSession  Re-scan
    |              (from state)   (fresh)     + Resume
    |
    +--- no state -----> RunSession (fresh, same as today)
```

### Resume Path (r)

1. `ReadState(baseDir)` — load state.json
2. `LoadScanResult(state.ScanResultPath)` — load cached ScanResult
3. `RestoreWaves(state.Waves)` — convert WaveState[] to Wave[]
4. `BuildCompletedWaveMap(waves)` — reconstruct completed set
5. Enter existing interactive loop (no changes to loop logic)

### New Session Path (n)

Same as current `RunSession`. Overwrites existing state.

### Re-scan Path (s)

1. `RunScan` — fresh scan against Linear
2. `RunWaveGenerate` — generate new waves
3. `MergeCompletedStatus(oldCompleted, newWaves)` — preserve completed wave status
4. `EvaluateUnlocks` — recompute prerequisites
5. Enter interactive loop

## Section 3: Navigator Resume Display

Add session status line to Navigator header when resuming:

```
+============================================================+
|              SIGHTJACK - Link Navigator                    |
|  Project: MyProject           |  Completeness:  62%        |
|  ADRs: 4                                                   |
|  Session: resumed (last scan: 2026-02-17 15:30)            |
+============================================================+
```

- New line: `Session: resumed (last scan: <timestamp>)`
- Only shown when resuming, not on fresh sessions

### PromptResume Display

```
  Previous session found (62% complete, 4 ADRs)
  Last scan: 2026-02-17 15:30

  [r] Resume session
  [n] Start new session
  [s] Re-scan Linear and resume

  Choice:
```

## Section 4: Re-scan Merge Strategy

### Principle

Completed waves remain completed. New scan results generate new waves for uncompleted work.

### Merge Logic

```go
func MergeCompletedStatus(oldCompleted map[string]bool, newWaves []Wave) []Wave
```

1. For each wave in newWaves, check if `WaveKey(wave)` exists in oldCompleted
2. If yes, mark as "completed"
3. If no, keep original status ("available" or "locked")
4. Waves that existed in old state but not in new scan are dropped (Linear removed them)

### Edge Cases

- **Same Wave ID, different content**: Completed status preserved (conservative). User can start new session if concerned.
- **New cluster appears**: All its waves start as "available"/"locked" per prerequisites.
- **Cluster disappears**: Its waves are simply absent from new generation.

## Out of Scope (v0.6+)

- Strictness level system (Fog/Alert/Lockdown)
- Shibito resurrection detection
- ADR contradiction detection
- Linear label-based state recovery
- Git-managed state with auto-commit
- Mid-wave checkpointing
