# Sightjack v0.3 Design — Architect Agent + Discussion Mode

**Date:** 2026-02-17
**Scope:** v0.3 — Architect Agent single-turn discussion, wave modification, re-approval loop
**Approach:** Single-turn Architect (Approach A) — one question, one response, wave modification

## Overview

v0.3 adds the Architect Agent to the wave approval flow. When the human selects "Discuss", they enter a free-text topic, the Architect Agent analyzes it in the context of the cluster and wave, and optionally returns a modified wave. The human then re-approves or continues discussing.

Key decisions:
- **Single-turn**: Each discussion round = one Claude subprocess call. No streaming, no --resume.
- **Discussion + Modify only**: Architect can modify wave actions. No ADR generation (deferred to v0.4).
- **Same subprocess pattern**: Uses `RunClaude` + JSON file output, identical to Pass 1-4.
- **Loopable**: Human can discuss multiple times before approving. Each round sees the latest wave state.
- **YAGNI**: No automatic "needs decision" detection. Human explicitly chooses to discuss.

## Architecture

```
Wave Approval Prompt
  [a] Approve  [r] Reject  [d] Discuss  [q] Back
                              |
                              v
                    Human enters topic (free text)
                              |
                              v
                    RunArchitectDiscuss(ctx, cfg, scanDir, wave, topic)
                      -> Render architect_discuss template
                      -> Claude subprocess (--print)
                      -> Output: architect_{cluster}_{wave}.json
                      -> Parse ArchitectResponse
                              |
                              v
                    Display: analysis text + reasoning
                    Display: modified wave diff (if any)
                              |
                              v
                    If modified_wave != nil:
                      Replace selected wave with modified version
                              |
                              v
                    Re-enter approval prompt with (possibly modified) wave
```

Legend:
- RunArchitectDiscuss: Architect Agent subprocess execution
- architect_discuss template: Go template for architect prompt
- ArchitectResponse: JSON output from Claude subprocess

## Data Model

### New Types

```go
// ArchitectResponse is the output of an architect discussion round.
type ArchitectResponse struct {
    Analysis     string `json:"analysis"`      // Architect's analysis text
    ModifiedWave *Wave  `json:"modified_wave"` // Modified wave proposal (nil = no changes)
    Reasoning    string `json:"reasoning"`     // Why changes were suggested
}
```

### CLI Types

```go
// ApprovalChoice represents the human's choice at the wave approval prompt.
type ApprovalChoice int

const (
    ApprovalApprove ApprovalChoice = iota
    ApprovalReject
    ApprovalDiscuss
    ApprovalQuit
)
```

### Prompt Data

```go
// ArchitectDiscussPromptData holds template data for the architect discussion prompt.
type ArchitectDiscussPromptData struct {
    ClusterName  string
    WaveTitle    string
    WaveActions  string // JSON-encoded current wave actions
    Topic        string // Human's free-text discussion topic
    OutputPath   string
}
```

## CLI Flow

### Updated Wave Approval

Before (v0.2):
```
  [a] Approve all  [r] Reject  [q] Back to navigator: _
```

After (v0.3):
```
  [a] Approve all  [r] Reject  [d] Discuss  [q] Back to navigator: _
```

### Discussion Sequence

```
Select wave [1-5, q=quit]: 1

--- Auth Cluster - Wave 1: Dependency Ordering ---
  Proposed actions (3):
    1. [add_dependency] ENG-101: Auth before token
    2. [add_dependency] ENG-102: Token before OAuth
    3. [add_dod] ENG-101: Auth flow sequence diagram

  Expected: 25% -> 40%

  [a] Approve all  [r] Reject  [d] Discuss  [q] Back to navigator: d

  Topic: _
  > Should we split ENG-101 into auth endpoints and middleware?

  [Architect] Analyzing Auth Cluster context...

  Analysis:
    Looking at the cluster as a whole, splitting would allow parallel
    implementation but adds management overhead for 23 issues.
    Recommendation: Keep unified, but add middleware interface to DoD.

  Reasoning:
    Project scale (23 issues) favors fewer issues. Middleware interface
    definition as DoD ensures API cluster can depend on it.

  Modified actions (3 -> 4):
    1. [add_dependency] ENG-101: Auth before token  (unchanged)
    2. [add_dependency] ENG-102: Token before OAuth  (unchanged)
    3. [add_dod] ENG-101: Auth flow sequence diagram  (unchanged)
  + 4. [add_dod] ENG-101: Middleware interface definition  (NEW)

  Expected: 25% -> 42%

  [a] Approve modified  [r] Reject  [d] Discuss again  [q] Back to navigator: a
```

## Function Signatures

### architect.go

```go
// RunArchitectDiscuss executes a single-turn architect discussion.
func RunArchitectDiscuss(ctx context.Context, cfg *Config, scanDir string, wave Wave, topic string) (*ArchitectResponse, error)

// ParseArchitectResult reads and parses an architect response JSON file.
func ParseArchitectResult(path string) (*ArchitectResponse, error)

// architectDiscussFileName returns the output filename for an architect discussion.
func architectDiscussFileName(wave Wave) string
```

### cli.go changes

```go
// PromptWaveApproval now returns ApprovalChoice instead of bool.
func PromptWaveApproval(ctx context.Context, w io.Writer, s *bufio.Scanner, wave Wave) (ApprovalChoice, error)

// PromptDiscussTopic reads a free-text discussion topic from the user.
func PromptDiscussTopic(ctx context.Context, w io.Writer, s *bufio.Scanner) (string, error)

// DisplayArchitectResponse shows the architect's analysis and any wave modifications.
func DisplayArchitectResponse(w io.Writer, resp *ArchitectResponse)
```

### prompt.go addition

```go
// RenderArchitectDiscussPrompt renders the architect discussion prompt.
func RenderArchitectDiscussPrompt(lang string, data ArchitectDiscussPromptData) (string, error)
```

## Session Loop Changes

The interactive loop in session.go gains a discuss branch:

```go
for {
    // ... existing wave selection ...

    // Inner approval loop (NEW: loops for discuss)
    for {
        choice, err := PromptWaveApproval(ctx, os.Stdout, scanner, selected)
        if err == ErrQuit { break }
        if err != nil { LogWarn(...); continue }

        switch choice {
        case ApprovalApprove:
            goto applyWave
        case ApprovalReject:
            goto nextWave
        case ApprovalDiscuss:
            topic, err := PromptDiscussTopic(ctx, os.Stdout, scanner)
            if err == ErrQuit { continue }
            result, err := RunArchitectDiscuss(ctx, cfg, scanDir, selected, topic)
            if err != nil { LogError(...); continue }
            DisplayArchitectResponse(os.Stdout, result)
            if result.ModifiedWave != nil {
                selected = *result.ModifiedWave
            }
            continue // back to approval with (possibly modified) wave
        case ApprovalQuit:
            goto nextWave
        }
    }
    // applyWave: ... existing apply logic ...
    // nextWave: continue
}
```

## File Structure

### New Files

```
architect.go                                    # RunArchitectDiscuss, ParseArchitectResult
architect_test.go                               # TDD tests
prompts/templates/architect_discuss_en.md.tmpl  # English architect prompt
prompts/templates/architect_discuss_ja.md.tmpl  # Japanese architect prompt
```

### Modified Files

```
model.go       # Add ArchitectResponse, ApprovalChoice
cli.go         # PromptWaveApproval returns ApprovalChoice, add PromptDiscussTopic, DisplayArchitectResponse
cli_test.go    # Update existing approval tests, add discuss tests
prompt.go      # Add ArchitectDiscussPromptData, RenderArchitectDiscussPrompt
prompt_test.go # Add architect template test
session.go     # Add discuss branch to interactive loop
session_test.go # Add discuss integration test
```

### Scan Directory (NEW output)

```
.siren/scans/{session-id}/
  architect_auth_auth-w1.json    # Architect discussion output
```

## Testing Strategy

TDD approach per component:

1. **model.go**: ArchitectResponse JSON marshal/unmarshal roundtrip
2. **prompt.go**: RenderArchitectDiscussPrompt with en/ja templates
3. **cli.go**:
   - PromptWaveApproval returns ApprovalDiscuss for "d" input
   - PromptWaveApproval returns ApprovalApprove for "a" (backward compat)
   - PromptWaveApproval returns ApprovalReject for "r" (backward compat)
   - PromptDiscussTopic reads free text line
   - PromptDiscussTopic handles quit
   - DisplayArchitectResponse with modifications
   - DisplayArchitectResponse without modifications
4. **architect.go**:
   - ParseArchitectResult valid JSON
   - ParseArchitectResult with nil modified_wave
   - RunArchitectDiscuss dry-run generates prompt file
5. **session.go**: Full discuss sequence (select -> discuss -> approve)

## Backward Compatibility

PromptWaveApproval signature change from `(bool, error)` to `(ApprovalChoice, error)` is breaking. All callers (session.go, tests) must be updated in the same commit.

## Future (NOT in v0.3)

- v0.4: Scribe Agent detects design decisions in architect dialog and generates ADRs
- v0.4: ADR → Wave feedback loop (new actions injected based on ADR)
- v0.5: Multi-turn discussion with context accumulation
