# Sightjack

**An interactive session that sends AI agents to sightjack your Linear issues — seeing their blind spots, ordering their dependencies, and architecting them until every issue is ready for autonomous execution.**

Sightjack uses [Claude Code](https://docs.anthropic.com/en/docs/claude-code) to analyze Linear issues across clusters, detect missing DoD (Definition of Done), hidden dependencies, and technical debt resurrection — then guides you through wave-by-wave approval to bring issue completeness from ~30% to ~85%.

```bash
sightjack session
```

This single command makes Sightjack repeat the following cycle:

1. Scan all Linear issues and classify them into thematic clusters
2. Deep-scan each cluster for completeness gaps and hidden dependencies
3. Generate execution waves with prerequisites and completeness deltas
4. Present the Link Navigator — you choose which wave to tackle
5. Review, discuss with the Architect agent, approve or modify
6. Apply changes to Linear, unlock dependent waves, repeat
7. Stop when all issues reach target completeness

## Why "Sightjack"?

The system design is inspired by the core mechanic of [Forbidden Siren](https://en.wikipedia.org/wiki/Siren_(video_game)) (SIREN), a PS2 survival horror game by Japan Studio (2003).

In the game, the player has a unique ability called "Sightjack" — tuning into enemies' visual perspectives like a radio dial, seeing through their eyes to discover threats hidden by fog and darkness. The game's Link Navigator lets you freely choose scenarios across characters and timelines, with completion of one scenario unlocking others.

This structure maps directly to issue architecture:

| Game Concept | Sightjack | Design Meaning |
|---|---|---|
| **Sightjack** | AI cross-issue analysis | See through each issue's perspective to find blind spots |
| **Link Navigator** | Matrix Navigator UI | Cluster x Wave matrix — choose your next scenario |
| **Scenario** | Wave | AI-generated batch of actions per cluster |
| **Character** | Issue Cluster | Thematic group (Auth, API, DB, Frontend...) |
| **Scenario Unlock** | Wave prerequisites | Completing Wave A unlocks Wave B |
| **End Condition 1** | Functional DoD | Basic acceptance criteria fulfilled |
| **End Condition 2** | Non-functional DoD | Hidden requirements the AI discovers |
| **Archive** | ADR (Architecture Decision Record) | Design decisions auto-generated during discussion |
| **Shibito Revival** | Technical debt detection | Closed issues whose patterns resurface in current work |
| **Fog of War** | Undiscovered requirements | New waves emerge as earlier ones complete |

### Three Design Principles

1. **Sightjack, don't guess** — AI scans every issue's perspective. Hidden dependencies and missing DoD are discovered, not assumed.
2. **Wave by wave, not issue by issue** — Decisions are made in meaningful batches (waves), not per-issue micro-dialogs. Like choosing a scenario in SIREN, not individual steps.
3. **Ripples reveal the whole** — Each wave completion shows ripple effects across clusters. The "aha, that's connected!" moment is the core experience.

---

## Game Mechanics

Three SIREN-inspired mechanics control session quality:

### Shibito Detection (Technical Debt Revival)

Past closed issues are scanned for patterns that resurface in current work. Like Shibito in SIREN that revive after being defeated.

```
[Sightjack] Shibito Revival Detected:
  ENG-045 (closed) -> ENG-201 (current)
  "Token circular dependency" — occurred in old auth, risk of recurrence
```

- Warnings shown once at session start
- Count tracked in state for session awareness

### Strictness Levels (Difficulty)

Controls how aggressively the AI enforces DoD completeness. Like SIREN's difficulty affecting enemy awareness.

```
fog       -> DoD gaps shown as warnings only (prototype/spike)
alert     -> Missing must-have DoD triggers sub-issue proposals
lockdown  -> All DoD required, dependencies enforced as blocked
```

- Configurable per-project and per-label override
- Affects wave generation: strict mode adds final consistency waves

### Wave Dynamic Evolution (Nextgen)

After each wave completion, the AI generates follow-up waves based on what was learned. Like SIREN's scenarios that only appear after certain conditions are met.

```
[Wave Complete] Auth W1: Dependency Ordering
  -> New wave generated: Auth W2: "JWT vs Session ADR" (emerged from W1 analysis)
  -> API W2 prerequisites updated (now depends on Auth W2)
```

## Architecture

```
Sightjack (binary)
    |
    |  Pass 1: Classify
    |  +-- Claude Code: cluster issues by theme
    |
    |  Pass 2: DeepScan
    |  +-- goroutines: parallel per-cluster analysis
    |  +-- semaphore: bounded concurrency
    |
    |  Pass 3: WaveGenerate
    |  +-- Claude Code: generate waves per cluster
    |  +-- EvaluateUnlocks: prerequisite chain resolution
    |
    v
Matrix Navigator (interactive)
    |
    |  Per Wave:
    |  +-- PromptWaveSelection -> PromptWaveApproval
    |  +-- Architect Agent: design discussion (optional)
    |  +-- Scribe Agent: ADR generation (optional)
    |  +-- Selective approval: pick individual actions
    |
    v
Pass 4: WaveApply
    |  +-- Claude Code: apply changes to Linear via MCP
    |  +-- Ripple effects displayed
    |  +-- Nextgen: dynamic follow-up wave generation
    |  +-- State saved (crash resilience)
    |
    v
Linear (via MCP Server)
    +-- Issues updated (DoD, dependencies, sub-issues)
    +-- ADRs stored as documents
    +-- Ready labels applied
```

Legend:
- Classify: Issue classification into thematic clusters
- DeepScan: Per-cluster completeness and dependency analysis
- WaveGenerate: Ordered execution wave creation
- WaveApply: Approved wave changes applied to Linear
- Matrix Navigator: Cluster x Wave interactive selection UI
- Architect Agent: Design discussion support
- Scribe Agent: ADR auto-generation from design decisions
- MCP Server: Model Context Protocol for Linear API access

### AI Agent Team

| Agent | Role | Game Concept |
|-------|------|-------------|
| Scanner | Classify + DeepScan + WaveGenerate | Sightjack (seeing through issues) |
| Architect | Design discussion during wave approval | Character dialogue |
| Scribe | ADR generation from design decisions | Archive collection |
| (Handoff) | Ready-issue labeling for downstream tools | Next expedition |

## Setup

```bash
# Build and install
go install github.com/hironow/sightjack/cmd/sightjack@latest

# Create config
cat > sightjack.yaml <<EOF
linear:
  team: "ENG"
  project: "My Project"
lang: "en"
EOF

# Run — .siren/ is created automatically
sightjack session
```

Sightjack creates `.siren/` and all state files automatically at runtime.

## Usage

```bash
# Scan only (classify + deep-scan, no interactive session)
sightjack scan

# Full interactive session (scan + wave approval + apply)
sightjack session

# Display last scan results
sightjack show

# Dry run (generate prompts without executing Claude)
sightjack scan --dry-run
sightjack session --dry-run

# Japanese prompts
sightjack session --lang ja

# Custom config path
sightjack session --config custom.yaml

# Verbose logging
sightjack session --verbose
```

## Options

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--config` | `-c` | `sightjack.yaml` | Config file path |
| `--lang` | `-l` | config (`ja`) | Language override (`en` / `ja`) |
| `--verbose` | `-v` | `false` | Verbose logging |
| `--dry-run` | | `false` | Generate prompts without executing Claude |
| `--version` | | | Show version and exit |

## Configuration

```yaml
# sightjack.yaml
linear:
  team: "ENG"            # Linear team key
  project: "My Project"  # Linear project name
  cycle: ""              # Optional: filter by cycle

scan:
  chunk_size: 20         # Issues per scan chunk
  max_concurrency: 3     # Parallel scan workers

claude:
  command: "claude"      # Claude CLI command
  model: ""              # Model override (optional)
  timeout_sec: 300       # Per-invocation timeout

scribe:
  enabled: true          # ADR generation via Scribe agent

strictness:
  default: "fog"         # Default strictness (fog/alert/lockdown)

retry:
  max_attempts: 3        # Retry on Claude failures
  base_delay_sec: 2      # Base backoff between retries

labels:
  enabled: true          # Auto-label ready issues in Linear

dod_templates:           # Custom DoD templates by issue type
  api_endpoint:
    must:
      - "Error responses (4xx/5xx) defined"
      - "Auth/authz requirements specified"
    should:
      - "Rate limiting documented"

lang: "ja"               # Language (en/ja)
```

## File Structure

```
+-- cmd/sightjack/
|   +-- main.go              CLI entry point + subcommand routing
+-- scanner.go               Scanner Agent (classify + deep-scan)
+-- architect.go             Architect Agent (design discussion)
+-- scribe.go                Scribe Agent (ADR generation)
+-- handoff.go               Handoff interface for downstream tools
+-- session.go               Session lifecycle (run, resume, rescan)
+-- wave.go                  Wave model + unlock evaluation
+-- wave_generator.go        Wave generation + nextgen (dynamic evolution)
+-- navigator.go             Matrix Navigator rendering
+-- cli.go                   Interactive prompts (selection, approval, discuss)
+-- claude.go                Claude Code subprocess runner
+-- config.go                sightjack.yaml parser + defaults
+-- model.go                 Core types (Cluster, Issue, Wave, Action, etc.)
+-- state.go                 State persistence (.siren/state.json)
+-- prompt.go                Go template renderer for AI prompts
+-- logger.go                Colored logging (LogOK, LogWarn, LogError, LogInfo)
+-- *_test.go                Tests (227+)
+-- prompts/
    +-- templates/
        +-- scanner_classify_{en,ja}.md.tmpl
        +-- scanner_deepscan_{en,ja}.md.tmpl
        +-- wave_generate_{en,ja}.md.tmpl
        +-- wave_apply_{en,ja}.md.tmpl
        +-- wave_nextgen_{en,ja}.md.tmpl
        +-- architect_discuss_{en,ja}.md.tmpl
        +-- scribe_adr_{en,ja}.md.tmpl
        +-- ready_label_{en,ja}.md.tmpl
```

## Prerequisites

- Go 1.25+
- [Claude Code CLI](https://docs.anthropic.com/en/docs/claude-code)
- [Linear MCP Server](https://github.com/anthropics/model-context-protocol) configured for Claude

## License

See [LICENSE](LICENSE) for details.
