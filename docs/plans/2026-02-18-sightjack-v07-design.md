# Sightjack v0.7 Design: UX Enhancement — SIREN Experience

**Goal:** Implement four UX features that bring the CLI experience closer to the SIREN game feel: matrix navigator, progress bar, "go back" to completed waves, and rich ripple effect display.

**Approach:** Renderer-focused (Approach A). All four features are presentation and input-handling changes. No data model changes required. session.go gets minimal modifications for the "go back" command routing.

**Tech Stack:** Go, Pure ASCII rendering (no Unicode box drawing), existing session loop

---

## Section 1: Matrix Navigator

Replace the current `RenderNavigatorWithWaves` flat grid with a true ASCII matrix using `+--+` borders.

### Output Format

```
+--------------------+--------+--------+--------+--------+------+
| Cluster            | W1     | W2     | W3     | W4     | Comp |
+--------------------+--------+--------+--------+--------+------+
| Auth (4)           | [=]    | [=]    | [ ]    |        |  65% |
| API (6)            | [=]    | [ ]    |        |        |  58% |
| DB (3)             | [=]    | [ ]    |        |        |  55% |
| Frontend (7)       | [=]    | [x]    |        |        |  42% |
| Infra (3)          | [=]    | [ ]    |        |        |  60% |
+--------------------+--------+--------+--------+--------+------+
  [=] completed  [ ] available  [x] locked

  [========..........] 62%  |  ADR: 4  |  Strictness: fog
```

### Implementation

- New function `RenderMatrixNavigator` replaces `RenderNavigatorWithWaves`
- Column widths: Cluster=20, Wave=8, Comp=6 (all fixed)
- Header row: `SIGHTJACK - Link Navigator` centered above the grid
- Project name and session info rendered above the grid
- Progress bar + metadata inline below the legend
- Wave cells: `[=]` completed, `[ ]` available, `[x]` locked, empty = not generated

---

## Section 2: Progress Bar

Pure function `RenderProgressBar(current float64, width int) string`.

### Output Format

```
[========..........] 62%
```

- `=` for completed portion, `.` for remaining
- Integrated into the matrix navigator footer
- Width parameter controls the bar length (default 20)

---

## Section 3: "Go Back" Experience

Add `[b]` option to `PromptWaveSelection` for revisiting completed waves.

### Flow

1. User presses `b` at wave selection
2. `PromptCompletedWaveSelection` shows list of completed waves
3. `DisplayCompletedWaveActions` shows what was applied
4. User can `[d]` to discuss modifications with Architect Agent
5. If architect produces `ModifiedWave`, re-apply via existing `RunWaveApply`

### Output Format

```
  Completed waves:
    1. Auth  W1: Dependency Ordering  (25% -> 40%)
    2. Auth  W2: DoD + ADR            (40% -> 65%)
    3. API   W1: Endpoint Split       (30% -> 45%)

  Select [1-3, q=back]: 2

  --- Auth - DoD + ADR (completed) ---
  Actions applied (3):
    1. [add_dod] ENG-101: Auth flow sequence diagram
    2. [add_dod] ENG-102: Token lifecycle definition
    3. [add_dependency] ENG-102 -> ENG-108

  [d] Discuss modifications  [q] Back to navigator:
```

### Design Decisions

- "Go back" enters the existing Architect Discuss loop. No new agent.
- Re-applying a modified completed wave does NOT reset its "completed" status.
- If modifications are made, the wave actions are updated in the waves slice via `PropagateWaveUpdate`.
- The completed wave's actions are available from the `Wave.Actions` field (already persisted in WaveState).

---

## Section 4: Rich Ripple Display

Enhance `DisplayRippleEffects` to group by cluster and show completeness delta.

### Output Format

```
  Wave completed: Auth W2 - DoD + ADR
  Completeness: 40% -> 65%

  Ripple effects:
    DB Cluster:
      -> Wave 2 unlocked (was waiting on Auth W2)
    API Cluster:
      -> ENG-205 DoD updated: "Token validation per ADR-007"

  New waves available: 2
  [========........] 52%
```

### Implementation

- `DisplayWaveCompletion` — new function combining completion banner + ripples + progress
- Groups ripples by `Ripple.ClusterName`
- Counts newly available waves (compare available count before/after)
- Shows progress bar at the end

---

## Files to Modify

| File | Change |
|------|--------|
| navigator.go | Replace `RenderNavigatorWithWaves` with `RenderMatrixNavigator`, add `RenderProgressBar` |
| navigator_test.go | Update all existing tests, add matrix format tests, progress bar tests |
| cli.go | Add `PromptCompletedWaveSelection`, `DisplayCompletedWaveActions`, `DisplayWaveCompletion` |
| cli_test.go | Add tests for new display/prompt functions |
| session.go | Add `[b]` routing in interactive loop, wire `DisplayWaveCompletion` |
| session_test.go | Update session state tests if needed |
| cmd/sightjack/main.go | Version bump to 0.7.0-dev |

## Non-Goals

- No data model changes (ScanResult, Wave, SessionState unchanged)
- No new AI agent or prompt template changes
- No config changes
- No "selective wave approval" (future scope)
- No dynamic wave regeneration (future scope)
