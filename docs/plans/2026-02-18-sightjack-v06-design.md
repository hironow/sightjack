# Sightjack v0.6 Design: SIREN Mechanics

**Goal:** Implement the three deferred SIREN mechanics — strictness levels (Fog/Alert/Lockdown), Shibito resurrection detection, and ADR contradiction checking — with strictness as the foundational layer.

**Architecture:** StrictnessLevel type as the core abstraction, injected into all AI prompts to control analysis depth. Shibito detection via Scanner prompt extension (no new subprocess). ADR consistency check via Scribe prompt extension with existing ADR injection.

**Tech Stack:** Go, Claude Code subprocess, Go templates, YAML config

---

## Design Decisions

- **Strictness default:** `fog` (gentlest level)
- **Strictness switch:** YAML config only (no CLI override, no label overrides)
- **Shibito data source:** Scanner prompt extension — AI uses Linear MCP to check closed issues
- **ADR conflict behavior:** Warning only at all levels (no blocking in v0.6)
- **Approach:** Strictness-first (Approach A) — strictness is the foundation, other features build on it

---

## Section 1: StrictnessLevel Type & Config

### New Types (model.go)

```go
type StrictnessLevel string

const (
    StrictnessFog      StrictnessLevel = "fog"
    StrictnessAlert    StrictnessLevel = "alert"
    StrictnessLockdown StrictnessLevel = "lockdown"
)

func ParseStrictnessLevel(s string) (StrictnessLevel, error)
func (l StrictnessLevel) Valid() bool
```

### Config Extension (config.go)

```go
type StrictnessConfig struct {
    Default StrictnessLevel `yaml:"default"`
}
```

Add `Strictness StrictnessConfig` to `Config`. Default: `StrictnessFog`.

### Validation

- `ParseStrictnessLevel` returns error for unknown strings
- `LoadConfig` with empty/missing strictness section uses fog default
- `LoadConfig` with invalid strictness value returns error

---

## Section 2: Prompt Strictness Injection

### PromptData Extensions

Add `StrictnessLevel string` field to:
- `ScanPromptData` (prompt.go)
- `WavePromptData` (prompt.go)
- `ArchitectPromptData` (prompt.go)
- `ScribeADRPromptData` (prompt.go)

### Template Changes

Each template gets a strictness instruction block:

```
## Strictness Level: {{.StrictnessLevel}}
- fog: Report DoD gaps as warnings. NFR issues are informational only.
- alert: Propose sub-issues for Must-level DoD gaps. NFR gets dedicated wave suggestions.
- lockdown: Flag ALL DoD gaps. Mark incomplete issues as blocked candidates.

Current level: {{.StrictnessLevel}}
```

### Session Integration

`session.go` passes `cfg.Strictness.Default` to all prompt rendering calls.

---

## Section 3: Shibito Resurrection Check

### Approach

No new agent/subprocess. Extend Scanner Agent's scan prompt to instruct AI to check for resurrection patterns using Linear MCP.

### Template Extension (scanner_scan.md.tmpl)

```
## Shibito Resurrection Check
Examine closed/cancelled issues in this project for patterns that
resemble current open issues. If you detect potential "resurrection"
(a previously resolved problem re-emerging), report it as a
shibito_warnings array in your JSON output.
```

### New Types (model.go)

```go
type ShibitoWarning struct {
    ClosedIssueID  string `json:"closed_issue_id"`
    CurrentIssueID string `json:"current_issue_id"`
    Description    string `json:"description"`
    RiskLevel      string `json:"risk_level"` // "low" | "medium" | "high"
}
```

Add to `ScanResult`:
```go
ShibitoWarnings []ShibitoWarning `json:"shibito_warnings,omitempty"`
```

Add to `SessionState`:
```go
ShibitoCount int `json:"shibito_count,omitempty"`
```

### Display

- Navigator header: `Shibito: N warnings` (when N > 0)
- Wave selection: Show relevant warnings for the selected cluster
- CLI display function: `DisplayShibitoWarnings(w io.Writer, warnings []ShibitoWarning)`

---

## Section 4: ADR Consistency Check

### Approach

Extend Scribe Agent's ADR generation prompt to include existing ADR content. AI checks for contradictions and reports them.

### Scribe Extension (scribe.go)

```go
func ReadExistingADRs(adrDir string) ([]ExistingADR, error)

type ExistingADR struct {
    Filename string
    Content  string
}
```

### Template Extension (scribe_adr.md.tmpl)

```
## ADR Consistency Check
Review existing ADRs below for potential contradictions with the
new decision being recorded. Report any conflicts as a "conflicts"
array in your JSON output.

{{range .ExistingADRs}}
### {{.Filename}}
{{.Content}}
{{end}}
```

### New Types (model.go)

```go
type ADRConflict struct {
    ExistingADRID string `json:"existing_adr_id"`
    Description   string `json:"description"`
}
```

Add to `ScribeResponse`:
```go
Conflicts []ADRConflict `json:"conflicts,omitempty"`
```

### Display

- After ADR generation: `[Scribe] Warning: Potential conflict with ADR-NNNN: <description>`
- CLI function: `DisplayADRConflicts(w io.Writer, conflicts []ADRConflict)`

---

## Affected Files Summary

| File | Changes |
|------|---------|
| model.go | StrictnessLevel type, ShibitoWarning, ADRConflict, ScanResult/SessionState/ScribeResponse extensions |
| config.go | StrictnessConfig, Config.Strictness field, DefaultConfig update |
| prompt.go | PromptData extensions (StrictnessLevel field), ScribeADRPromptData.ExistingADRs |
| scanner_scan*.tmpl | Strictness block + Shibito resurrection instructions |
| scanner_wave*.tmpl | Strictness block |
| architect*.tmpl | Strictness block |
| scribe_adr*.tmpl | Strictness block + ADR consistency check + existing ADRs |
| scribe.go | ReadExistingADRs, pass to prompt, parse conflicts from response |
| session.go | Pass strictness level to all prompts, display shibito warnings, display ADR conflicts |
| navigator.go | Strictness badge, Shibito count in header |
| cli.go | DisplayShibitoWarnings, DisplayADRConflicts |
| state.go | ShibitoCount persistence (if needed) |
