# Sightjack v0.8 Design: Wave Dynamic Evolution

## Overview

v0.8 adds three interconnected features that bring Sightjack closer to the SIREN game's dynamic scenario evolution:

1. **Wave Dynamic Generation** — Auto-generate new waves after wave completion
2. **ADR Feedback** — Inject existing ADRs into wave generation prompts
3. **Selective Approval** — Approve/reject individual wave actions

These features transform the static wave set into a living, evolving system where completing work creates new work, past decisions inform future proposals, and humans have fine-grained control over what gets applied.

## A: Wave Dynamic Generation

### Concept

When a wave is completed, Sightjack automatically generates the next wave for that cluster. This mirrors SIREN's scenario unlock mechanic — clearing one scenario reveals new ones.

### Design

New file: `wave_generator.go`

```
Wave completed (session.go:240)
    |
    v
GenerateNextWaves(ctx, cfg, scanDir, clusterName, completedWaves, existingADRs, rejectedActions)
    |
    v
Claude subprocess with wave_nextgen_{lang}.md.tmpl
    |
    v
ParseNextGenResult → []Wave (0 or more new waves)
    |
    v
Append to session waves[], update navigator
```

**Key types:**

```go
// NextGenPromptData in prompt.go
type NextGenPromptData struct {
    ClusterName      string
    Completeness     string
    Issues           string
    CompletedWaves   string     // JSON summary of completed waves
    ExistingADRs     []ExistingADR
    RejectedActions  string     // Actions user rejected in selective approval
    OutputPath       string
    StrictnessLevel  string
}

// NextGenResult in model.go
type NextGenResult struct {
    ClusterName string `json:"cluster_name"`
    Waves       []Wave `json:"waves"`
    Reasoning   string `json:"reasoning"`
}
```

**Integration point (session.go ~line 262):**
After `DisplayWaveCompletion`, call `GenerateNextWaves`. New waves are appended to the `waves` slice with `status: "available"` or `status: "locked"`.

**Dry-run:** Generate a sample nextgen prompt file alongside existing dry-run outputs.

### Constraints

- Maximum 2 new waves per generation (prevent explosion)
- New waves get IDs like `{cluster}-w{N+1}` where N is the current max wave number for that cluster
- If Claude returns 0 waves, that's valid (cluster may be complete)

## B: ADR Feedback (Prompt Injection)

### Concept

Existing ADRs are included in the wave generation prompt so the AI considers past design decisions when proposing new work.

### Design

`ReadExistingADRs(adrDir)` already exists in `scribe.go:89`. Reuse it directly.

**Changes:**

1. Add `ExistingADRs []ExistingADR` field to `NextGenPromptData` (already included above)
2. Template `wave_nextgen_{lang}.md.tmpl` includes an ADR section:
   ```
   {{if .ExistingADRs}}
   ## Existing ADRs (design decisions to respect)
   {{range .ExistingADRs}}
   ### {{.Filename}}
   {{.Content}}
   {{end}}
   {{end}}
   ```
3. No new Go code beyond the template and prompt data field

### Constraints

- ADR content is included verbatim (no summarization)
- If ADR directory is empty, the section is omitted from the prompt
- ADR read errors are non-fatal (LogWarn, proceed without ADRs)

## C: Selective Approval

### Concept

Instead of all-or-nothing wave approval, users can toggle individual actions on/off. Rejected actions are recorded and fed back into the next wave generation.

### Design

**New ApprovalChoice:**

```go
// model.go
const (
    ApprovalApprove ApprovalChoice = iota
    ApprovalReject
    ApprovalDiscuss
    ApprovalQuit
    ApprovalSelective  // NEW
)
```

**New CLI function:**

```go
// cli.go
func PromptSelectiveApproval(ctx context.Context, w io.Writer, s *bufio.Scanner, wave Wave) ([]WaveAction, []WaveAction, error)
// Returns: (approved actions, rejected actions, error)
```

**UI Flow:**

```
--- Auth - JWT middleware ---
  Proposed actions (3):
    1. [x] [add_dod] ENG-101: Add error handling spec
    2. [x] [add_dep] ENG-102: Depends on ENG-101
    3. [x] [create]  ENG-103: Refresh token sub-issue

  Toggle [1-3, a=all, n=none, done=confirm]: 3
    3. [ ] [create]  ENG-103: Refresh token sub-issue

  Toggle [1-3, a=all, n=none, done=confirm]: done

  Applying 2 of 3 actions...
```

**Approval prompt update (cli.go:78):**

```
  [a] Approve all  [s] Selective  [r] Reject  [d] Discuss  [q] Back to navigator:
```

**Session integration (session.go ~line 176):**

```go
case ApprovalSelective:
    approved, rejected, selErr := PromptSelectiveApproval(ctx, os.Stdout, scanner, selected)
    if selErr != nil { ... }
    if len(approved) == 0 {
        LogInfo("No actions selected. Wave skipped.")
        break
    }
    // Build partial wave with only approved actions
    partialWave := selected
    partialWave.Actions = approved
    selected = partialWave
    // Record rejected for next wave generation
    sessionRejected[WaveKey(selected)] = rejected
    applyWave = true
```

**Rejected actions storage:**

- `sessionRejected map[string][]WaveAction` — in-memory only, scoped to session
- Passed to `GenerateNextWaves` so AI knows "user rejected these, don't re-propose the same thing"
- Not persisted to state.json (ephemeral, session-scoped)

### Constraints

- Default state: all actions selected (checkbox on)
- `a` = select all, `n` = deselect all, `done` = confirm
- Invalid input re-prompts
- If all actions deselected and `done` pressed, treat as reject
