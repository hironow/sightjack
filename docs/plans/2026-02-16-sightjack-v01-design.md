# Sightjack v0.1 Skeleton Design

**Date:** 2026-02-16
**Scope:** v0.1 — Linear connection, Issue fetch, Cluster detection, Link Navigator display
**Approach:** Paintress Clone-and-Adapt with docs flat structure naming

## Overview

Sightjack v0.1 implements the minimal viable skeleton:

1. `sightjack scan` command triggers Scanner Agent via Claude Code subprocess
2. Claude Code fetches Issues from Linear (MCP), classifies into Clusters, evaluates completeness
3. Results written to files by Claude Code, read and parsed by Go
4. Link Navigator displayed as ASCII matrix in terminal
5. State persisted to `.sightjack/state.json`

## Architecture Decisions

- **Go is orchestrator only** — no direct Linear API calls from Go
- **File-based communication** — Claude Code writes JSON to specified paths; Go reads after process exit
- **Two-pass scanning** — Pass 1 classifies Issues into Clusters (lightweight); Pass 2 deep-scans per Cluster (parallel)
- **Paintress subprocess pattern** — `exec.CommandContext` with streaming goroutine for terminal output
- **Flat file structure** — following docs specification, no `internal/` package

## Project Structure

```
sightjack/
├── cmd/sightjack/main.go       # CLI entrypoint (flag package)
├── claude.go                    # Claude Code subprocess management
├── scanner.go                   # Scanner Agent: two-pass scan orchestration
├── navigator.go                 # Link Navigator ASCII rendering
├── state.go                     # State file read/write
├── config.go                    # sightjack.yaml parsing
├── model.go                     # Domain types
├── prompt.go                    # Template loading + rendering
├── logger.go                    # Colored logging
├── prompts/
│   └── templates/
│       ├── scanner_classify_ja.md.tmpl
│       ├── scanner_classify_en.md.tmpl
│       ├── scanner_deepscan_ja.md.tmpl
│       └── scanner_deepscan_en.md.tmpl
├── .sightjack/                  # Runtime state (gitignored)
│   ├── state.json
│   └── scans/{session-id}/
│       ├── classify.json
│       └── cluster_{name}.json
├── sightjack.yaml               # Project config
├── justfile
├── go.mod
└── tests/
```

## Domain Model (model.go)

```go
type Issue struct {
    ID          string
    Identifier  string   // e.g. "AWE-50"
    Title       string
    Description string
    State       string
    Labels      []string
    Priority    int
}

type Cluster struct {
    Name         string
    Issues       []Issue
    Completeness float64  // 0.0 - 1.0
    Gaps         []string // Per-issue gaps aggregated
    Observations []string
}

type ScanResult struct {
    Clusters     []Cluster
    TotalIssues  int
    Completeness float64
    Observations []string
}

type SessionState struct {
    Version       string
    ProjectID     string
    SessionID     string
    Completeness  float64
    Clusters      []ClusterState
    LastScanned   time.Time
}

type ClusterState struct {
    Name         string
    Completeness float64
    IssueCount   int
}
```

## Claude Code Subprocess (claude.go)

File-based communication pattern:

1. Go creates output directory: `.sightjack/scans/{sessionID}/`
2. Go launches Claude Code with prompt containing output file path
3. Claude Code analyzes Issues via Linear MCP, writes JSON to specified path
4. Go waits for process exit, then reads output file
5. Go parses JSON into domain types

Concurrency for Pass 2: semaphore-limited goroutines (default 3).

## Scanner Two-Pass Strategy (scanner.go)

**Pass 1 (Classify):**
- Prompt instructs Claude to fetch all Issues and classify into clusters
- Output: `classify.json` with cluster names and issue ID lists

**Pass 2 (Deep Scan):**
- Per-cluster prompts with full Issue details
- Parallel execution with configurable concurrency
- Output: `cluster_{name}.json` with completeness and gap analysis

## Link Navigator (navigator.go)

ASCII matrix display:

```
+==================================================+
|           SIGHTJACK - Link Navigator              |
|  Project: MyProject  |  Completeness: 32%         |
+==================================================+
|                                                    |
|  Cluster        W1    W2    W3    W4    Comp.     |
|  ------------------------------------------------ |
|  Auth           []    []    []    []    25%       |
|  API            []    []    []    []    40%       |
|  Database       []    []    []    []    35%       |
|  Frontend       []    []    []    []    30%       |
|  Infra          []    []    []    []    20%       |
|                                                    |
+==================================================+
|  [] not generated  [=] available  [#] complete    |
|  [x] locked (dependency)                          |
+==================================================+
```

v0.1: All wave cells show `[]` (not generated). Wave execution comes in v0.2.

## Configuration (sightjack.yaml)

```yaml
linear:
  team: "MY-TEAM"
  project: "My Project"
  cycle: ""

scan:
  chunk_size: 20
  max_concurrency: 3

claude:
  command: "claude"
  model: "opus"
  timeout_sec: 300

lang: "ja"
```

## CLI Commands

```bash
sightjack scan                    # Full scan (Pass 1 + Pass 2)
sightjack scan --config path.yaml # Custom config
sightjack show                    # Re-display last scan from state.json

# Flags
--config, -c    Config file path (default: sightjack.yaml)
--lang, -l      Language override
--verbose, -v   Verbose logging
--dry-run       Generate prompts only
```

## State File (.sightjack/state.json)

Thin state recoverable from Linear. Contains: version, session ID, project,
last scan timestamp, overall completeness, and per-cluster completeness.

## Testing Strategy

TDD approach:
- model.go: Unit tests for type construction and validation
- config.go: Unit tests for YAML parsing
- navigator.go: Unit tests for ASCII rendering
- state.go: Unit tests for state read/write
- scanner.go: Integration test with mock Claude output files
- claude.go: Integration test with --dry-run mode

## Future Extensions (not in v0.1)

- v0.2: Wave execution (dynamic generation, propose/approve/apply)
- v0.3: Architect Agent dialogue mode
- v0.4: ADR system (Scribe Agent)
- v0.5: Session persistence, strictness levels, resurrection detection
