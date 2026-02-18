# Sightjack v0.9 Design: Production Ready + Full Integration

## Overview

v0.9 addresses all remaining deferred features, bringing Sightjack to production-ready quality. Seven interconnected features complete the v1.0 roadmap:

1. **DoD Templates** (A) — Category-based Definition of Done templates in config
2. **Error Handling / Retry** (B) — Exponential backoff retry for Claude subprocess calls
3. **Completeness Tracking** (C) — Partial apply delta and final consistency check
4. **Linear Label Integration** (D) — `sightjack:` labels on Linear issues for visibility
5. **Paintress Bridge** (E) — `sightjack:ready` label for AI implementation agent pickup
6. **State Recovery** (F) — Recover session state from cached scan results
7. **Parallel Scan** (G) — Concurrent deep scan across clusters

## A: DoD Templates

### Concept

`sightjack.yaml` gains a `dod_templates` section with must/should DoD items per category. These are injected into wave generation prompts so the AI proposes DoDmatching project standards.

### Design

Add to `config.go`:
```go
type DoDTemplate struct {
    Must   []string `yaml:"must"`
    Should []string `yaml:"should"`
}
```

Config field: `DoDTemplates map[string]DoDTemplate` in `Config`.

Template injection: `wave_generate` and `wave_nextgen` templates gain a `{{if .DoDTemplates}}` section listing must/should items for the target cluster's category.

Category matching: cluster name is matched against template keys (case-insensitive prefix match). No match = AI decides freely.

## B: Error Handling / Retry

### Concept

Claude subprocess calls get exponential backoff retry for transient failures (process crash, timeout, non-zero exit).

### Design

Add to `config.go`:
```go
type RetryConfig struct {
    MaxAttempts int `yaml:"max_attempts"` // default: 3
    BaseDelay   time.Duration `yaml:"base_delay"` // default: 2s
}
```

Retry in `RunClaude` (claude.go):
- Attempt 1: immediate
- Attempt 2: 2s delay
- Attempt 3: 4s delay
- Retry target: process start failure, timeout, non-zero exit
- NOT retried: context cancellation (user Ctrl+C)
- Log: `LogInfo("Retrying (%d/%d)...", attempt, max)`

`RunClaudeDryRun` does not retry (file write only).

## C: Completeness Tracking

### Concept

Partial wave apply and final consistency checks improve completeness accuracy.

### Design

Add `AppliedCount` / `TotalCount` to `WaveApplyResult` if not present.

Partial apply: when wave apply partially succeeds (some errors), compute:
```
successRate = AppliedCount / TotalCount
partialDelta = Delta.Before + (Delta.After - Delta.Before) * successRate
```

Final consistency: on session end (quit or 85%), verify sum of cluster completeness matches overall. LogWarn on mismatch.

## D: Linear Label Integration

### Concept

Template-driven label assignment on Linear issues for session visibility.

### Design

Add to `config.go`:
```go
type LabelsConfig struct {
    Enabled    bool   `yaml:"enabled"`     // default: true
    Prefix     string `yaml:"prefix"`      // default: "sightjack"
    ReadyLabel string `yaml:"ready_label"` // default: "sightjack:ready"
}
```

Template changes:
- `scanner_classify` templates: instruct Claude to apply `sightjack:analyzed` label
- `wave_apply` templates: instruct Claude to apply `sightjack:wave-done` label
- Conditional on `{{if .LabelsEnabled}}`

No Go-side Linear API calls. Labels applied via Claude subprocess + Linear MCP.

## E: Paintress Bridge

### Concept

`sightjack:ready` label marks issues ready for AI implementation agent pickup.

### Design

Depends on Section D (label infrastructure).

Ready criteria (evaluated in Go):
- All waves targeting the issue are completed
- Issue has no remaining gaps (DoD defined)

Go-side: after wave completion, compute `ReadyIssueIDs` list and pass to template.
Template: apply `sightjack:ready` label to listed issues.

Issue-level (not wave-level): label applied when the LAST wave touching that issue completes.

## F: State Recovery

### Concept

Recover session state when `.siren/state.json` is missing or corrupted.

### Design

Recovery chain in `ResumeSession`:
1. Try `state.json` — normal resume
2. Try `scan_result.json` — recover clusters, waves, completeness
3. Neither exists — re-scan (new session)

```go
func RecoverStateFromScan(scanResult *ScanResult, waves []Wave, adrDir string) *SessionState
```

Recoverable: clusters, wave states, completeness, ADR count (from `docs/adr/` file count).
Unrecoverable (acceptable loss): sessionRejected (empty), exact lastScanned (use mtime).

Log: `LogWarn("State file missing. Recovered from cached scan result.")`

## G: Parallel Scan

### Concept

Concurrent deep scan across clusters using goroutines with configurable concurrency.

### Design

Add to `config.go`:
```go
type ExecutionConfig struct {
    MaxConcurrency int `yaml:"max_concurrency"` // default: 3
}
```

New function in `scanner.go`:
```go
func RunParallelDeepScan(ctx context.Context, cfg *Config, scanDir string,
    clusters []ClusterScanResult, dryRun bool) ([]ClusterScanResult, error)
```

Implementation: semaphore channel for concurrency control. Individual error collection (not errgroup) — failed clusters get LogWarn, remaining continue.

Activation: cluster count >= 2 && MaxConcurrency > 1. Dry-run skips parallelization.

## Dependencies

| Section | Depends on |
|---------|-----------|
| A: DoD Templates | — |
| B: Error Handling | — |
| C: Completeness | — |
| D: Linear Labels | — |
| E: Paintress Bridge | D |
| F: State Recovery | — |
| G: Parallel Scan | — (B recommended) |

## Constraints

- All features follow existing patterns: TDD, template-driven Claude prompts, non-fatal error handling
- Labels applied via Claude subprocess + Linear MCP (not direct API calls)
- Config additions use sensible defaults (zero-config works)
- Design doc will be deleted before merge (docs/ convention: current state only)
