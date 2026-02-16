# Sightjack v0.2 Design — Wave Generation + Execution

**Date:** 2026-02-17
**Scope:** v0.2 — Wave dynamic generation, propose/approve/apply flow, unlock conditions
**Approach:** 4-Pass Session extending v0.1's 2-Pass architecture

## Overview

v0.2 implements the SIREN experience loop: the player (human) sees the Link Navigator, picks a Wave, reviews AI's proposal, approves, and watches the ripple effects propagate. This is the core gameplay of Sightjack.

Key decisions:
- **4-Pass architecture**: Classify → DeepScan → Wave Generate → Wave Apply
- **Linear actual writes**: Claude Code applies approved changes via Linear MCP
- **Interactive CLI**: `sightjack session` command with stdin-based interaction
- **No Architect Agent**: Approve/reject only (discussion mode deferred to v0.3)
- **File-based communication**: Go ↔ Claude Code via JSON files (same as v0.1)

## Architecture: 4-Pass Session

```
+----------------------------------------------------+
|              sightjack session                      |
|                                                     |
|  Pass 1: Classify          (v0.1 reuse)             |
|    Claude fetches Issues, groups into Clusters      |
|    Output: classify.json                            |
|                                                     |
|  Pass 2: Deep Scan         (v0.1 reuse, parallel)   |
|    Per-cluster completeness + gaps analysis         |
|    Output: cluster_{name}.json                      |
|                                                     |
|  Pass 3: Wave Generate     (NEW, parallel)          |
|    Per-cluster wave proposal based on gaps           |
|    Output: wave_{name}.json                         |
|                                                     |
|  --- Human Interactive Loop ---                     |
|                                                     |
|  Display Link Navigator (with Waves)                |
|  Human selects a Wave                               |
|  Display Wave proposal                              |
|  Human approves / rejects                           |
|                                                     |
|  Pass 4: Wave Apply        (NEW, per-wave)          |
|    Claude applies approved actions to Linear         |
|    Output: apply_{wave_id}.json                     |
|                                                     |
|  Display ripple effects                             |
|  Update state                                       |
|  Loop back to Link Navigator                        |
+----------------------------------------------------+

Legend:
- Classify: Issue Classification
- Deep Scan: Completeness Gap Analysis
- Wave Generate: Wave Proposal Generation
- Wave Apply: Linear Issue Update
- Link Navigator: Interactive Selection Matrix
```

Pass 1-3 run automatically at session start. Pass 4 runs per approved wave in the interactive loop.

## Data Model

### Wave Types (NEW)

```go
type Wave struct {
    ID            string       `json:"id"`
    ClusterName   string       `json:"cluster_name"`
    Title         string       `json:"title"`
    Description   string       `json:"description"`
    Actions       []WaveAction `json:"actions"`
    Prerequisites []string     `json:"prerequisites"`
    Delta         WaveDelta    `json:"delta"`
    Status        string       `json:"status"` // "available" | "locked" | "completed"
}

type WaveAction struct {
    Type        string `json:"type"`    // "add_dod" | "add_dependency" | "add_label" | "update_description"
    IssueID     string `json:"issue_id"`
    Description string `json:"description"`
    Detail      string `json:"detail"`
}

type WaveDelta struct {
    Before float64 `json:"before"`
    After  float64 `json:"after"`
}

type WaveGenerateResult struct {
    ClusterName string `json:"cluster_name"`
    Waves       []Wave `json:"waves"`
}

type WaveApplyResult struct {
    WaveID  string   `json:"wave_id"`
    Applied int      `json:"applied"`
    Errors  []string `json:"errors"`
    Ripples []Ripple `json:"ripples"`
}

type Ripple struct {
    ClusterName string `json:"cluster_name"`
    Description string `json:"description"`
}
```

### State Extension

```go
type SessionState struct {
    Version      string         `json:"version"`       // "0.2"
    SessionID    string         `json:"session_id"`
    Project      string         `json:"project"`
    LastScanned  time.Time      `json:"last_scanned"`
    Completeness float64        `json:"completeness"`
    Clusters     []ClusterState `json:"clusters"`
    Waves        []WaveState    `json:"waves"`
}

type WaveState struct {
    ID            string   `json:"id"`
    ClusterName   string   `json:"cluster_name"`
    Title         string   `json:"title"`
    Status        string   `json:"status"`
    Prerequisites []string `json:"prerequisites"`
    ActionCount   int      `json:"action_count"`
}
```

## Interactive CLI Flow

```
$ sightjack session

[SCAN] Pass 1: Classifying issues...
[SCAN] Pass 2: Deep scanning 5 clusters...
[SCAN] Pass 3: Generating waves for 5 clusters...
[ OK ] 5 clusters, 12 waves generated

+==============================================================+
|             SIGHTJACK - Link Navigator                       |
|  Project: MyProject          |  Completeness:  32%           |
+==============================================================+
|                                                              |
|  Cluster         | W1          | W2          | Comp.         |
|  ---------------------------------------------------------- |
|  Auth (4)        | [ ] Deps    | [x] DoD     |  25%         |
|  API  (6)        | [ ] Split   | [x] locked  |  30%         |
|  DB   (3)        | [ ] Schema  | [x] locked  |  35%         |
|  Frontend (7)    | [ ] Comps   | [x] locked  |  28%         |
|  Infra (3)       | [ ] Env     | [x] locked  |  40%         |
|                                                              |
|  [ ] available  [x] locked  [=] completed                   |
+==============================================================+

Available waves:
  1. Auth  W1: Dependency Ordering           (25% -> 40%)
  2. API   W1: Endpoint Split                (30% -> 45%)
  3. DB    W1: Schema Dependency             (35% -> 50%)
  4. Front W1: Component Partition           (28% -> 42%)
  5. Infra W1: Environment Setup             (40% -> 55%)

Select wave [1-5, q=quit]: 1

--- Auth Cluster - Wave 1: Dependency Ordering ---
  Target: ENG-101, ENG-102, ENG-103, ENG-108

  Proposed actions (7):
    [Dependencies]
      1. ENG-101 -> ENG-102 (auth first, token after)
      2. ENG-101 -> ENG-103 (auth first, reset after)
      3. ENG-102 -> ENG-108 (token first, oauth after)
    [DoD additions]
      4. ENG-101: "Auth flow sequence diagram"
      5. ENG-102: "Token lifecycle state transition"
      6. ENG-108: "Supported OAuth2 provider list"
    [Labels]
      7. All issues: sightjack:wave1

  Expected: 25% -> 40%

  [a] Approve all  [r] Reject  [q] Back to navigator: a

[APPLY] Applying Auth W1 to Linear...
[ OK ] 7 actions applied successfully

  Ripple effects:
    -> API Cluster: W2 unlocked (Auth W1 was prerequisite)
    -> DB Cluster: W2 now available (Auth dependency resolved)

  Completeness: 32% -> 36%

Press Enter to continue...
```

Input handling: `bufio.Scanner` reads single lines from stdin.
- Wave selection: numeric input (1-N)
- Wave approval: single character (a/r/q)
- Quit: `q` at any prompt (state auto-saved)

## File Structure

### Scan Directory

```
.siren/
  state.json
  scans/{session-id}/
    classify.json              # Pass 1
    cluster_00_auth.json       # Pass 2
    cluster_01_api.json
    wave_00_auth.json          # Pass 3 (NEW)
    wave_01_api.json
    apply_auth-w1.json         # Pass 4 (NEW)
```

### Prompt Templates

```
prompts/templates/
  scanner_classify_ja.md.tmpl   # Pass 1 (v0.1)
  scanner_classify_en.md.tmpl
  scanner_deepscan_ja.md.tmpl   # Pass 2 (v0.1)
  scanner_deepscan_en.md.tmpl
  wave_generate_ja.md.tmpl      # Pass 3 (NEW)
  wave_generate_en.md.tmpl
  wave_apply_ja.md.tmpl         # Pass 4 (NEW)
  wave_apply_en.md.tmpl
```

### Source Files

```
cmd/sightjack/main.go          # MODIFY: Add "session" subcommand
session.go                      # NEW: Session loop orchestration
wave.go                         # NEW: Wave types, parsing, unlock logic
cli.go                          # NEW: stdin input, prompt display
navigator.go                    # MODIFY: Add wave columns to display
model.go                        # MODIFY: Add Wave types
state.go                        # MODIFY: WaveState in SessionState
scanner.go                      # MODIFY: Pass 3 integration
prompt.go                       # MODIFY: Wave template rendering
```

## Wave Unlock Logic

A wave's status is determined by its prerequisites:

```go
func (w *Wave) IsAvailable(completedWaves map[string]bool) bool {
    for _, prereq := range w.Prerequisites {
        if !completedWaves[prereq] {
            return false
        }
    }
    return true
}
```

After each Wave apply, re-evaluate all locked waves. Newly unlocked waves are included in the next Link Navigator display. This creates the SIREN "scenario unlock" experience.

## Testing Strategy

TDD approach per component:
- wave.go: Unit tests for Wave parsing, unlock logic, merge
- session.go: Unit tests with mock input reader (no stdin in tests)
- cli.go: Unit tests with bytes.Buffer for input/output
- navigator.go: Unit tests for wave-column rendering
- model.go: Unit tests for new types
- prompt.go: Unit tests for wave template rendering
- Integration: dry-run mode for Pass 3/4

## Future Extensions (not in v0.2)

- v0.3: Architect Agent dialogue mode (discuss before approve)
- v0.4: Scribe Agent + ADR generation
- v0.5: Session persistence, strictness levels, resurrection detection
