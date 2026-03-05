# Sightjack

**An interactive session that sends AI agents to sightjack your Linear issues — seeing their blind spots, ordering their dependencies, and architecting them until every issue is ready for autonomous execution.**

Sightjack uses [Claude Code](https://docs.anthropic.com/en/docs/claude-code) to analyze Linear issues across clusters, detect missing DoD (Definition of Done), hidden dependencies, and technical debt resurrection — then guides you through wave-by-wave approval to bring issue completeness from ~30% to ~85%.

```bash
sightjack run
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

## D-Mail & Convergence Gate

Sightjack communicates with downstream tools (amadeus, paintress) via D-Mail, a file-based message protocol. D-Mail kinds:

| Kind | Direction | Description |
|------|-----------|-------------|
| `specification` | outbound | Wave specification sent to downstream tools |
| `report` | outbound | Wave completion report |
| `feedback` | inbound | Feedback from downstream tools (injected into nextgen prompts) |
| `convergence` | inbound | Convergence signal — requires user approval before session proceeds |

When a `convergence` D-Mail is detected at session startup, the **convergence gate** activates:

1. **Notify** — desktop notification (fire-and-forget, non-blocking with 30s timeout)
2. **Approve** — blocking prompt (stdin y/N, external command, or auto-approve)
3. **Re-drain** — checks for late-arriving convergence and loops if found

The gate runs before the interactive wave loop in all session modes (`run`, `resume`, `rescan`). Gate behavior is configurable via `gate:` config section or `--notify-cmd` / `--approve-cmd` / `--auto-approve` CLI flags.

SKILL.md files in `.siren/skills/` declare produces/consumes routing for phonewave discovery using Agent Skills spec format with `dmail-schema-version: "1"`.

## Architecture

### Pipe Architecture (v0.0.12+)

```
scan --json ──→ waves ──→ select ──→ discuss ──→ apply ──→ nextgen
   |              |          |           |          |          |
   v              v          v           v          v          v
ScanResult    WavePlan     Wave    DiscussResult ApplyResult WavePlan
                                       |
                                       └──→ adr ──→ ADR Markdown
```

All data flows as JSON through Unix pipes. Each subcommand is a standalone filter:

- **stdin**: JSON from previous command (or file)
- **stdout**: JSON for next command (or file)
- **stderr**: all log output
- **/dev/tty**: interactive prompts (select, discuss)

### Monolithic Architecture

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
just install

# Initialize project config (Linear team key, etc.)
sightjack init

# Check environment
sightjack doctor

# Run — .siren/ is created automatically
sightjack run
```

Sightjack creates `.siren/` and all state/run files automatically at runtime.

## Subcommands

### Interactive

| Command | Description |
|---------|-------------|
| `sightjack scan` | Classify and deep-scan Linear issues (default when no subcommand given) |
| `sightjack run` | Interactive wave approval and apply loop (auto-resumes from state) |
| `sightjack show` | Display last scan results (or pipe JSON from stdin) |
| `sightjack init` | Initialize `.siren/config.yaml` interactively |
| `sightjack doctor` | Check environment and tool availability |
| `sightjack version` | Print version, commit, date, and Go version (`-j` for JSON) |
| `sightjack update` | Self-update to the latest GitHub release (`-C` to check only) |
| `sightjack status` | Show sightjack operational status |
| `sightjack clean` | Remove state directory (`.siren/`) |
| `sightjack archive-prune` | Remove expired scan archives (`-x` to execute, default: dry-run) |

### Pipe-friendly (Unix pipeline)

Each subcommand reads JSON from stdin and writes JSON to stdout. Logs go to stderr. Interactive prompts use `/dev/tty` (falls back to `CONIN$` on Windows).

| Command | stdin | stdout | Description |
|---------|-------|--------|-------------|
| `sightjack scan --json` | — | `ScanResult` | Scan and output structured JSON |
| `sightjack waves` | `ScanResult` | `WavePlan` | Generate execution waves |
| `sightjack select` | `WavePlan` | `Wave` | Interactive wave selection (tty) |
| `sightjack discuss` | `Wave` | `DiscussResult` | Architect discussion (tty) |
| `sightjack apply` | `Wave` | `ApplyResult` | Apply wave actions to Linear |
| `sightjack adr` | `DiscussResult` | ADR Markdown | Generate ADR document |
| `sightjack nextgen` | `ApplyResult` | `WavePlan` | Generate follow-up waves |
| `sightjack show` | `ScanResult` or `WavePlan` | human-readable | Render piped JSON for display |

## Usage

Flags and subcommand can be placed in any order. All flags support GNU/POSIX long (`--flag`) and short (`-f`) forms:

```bash
sightjack scan --dry-run         # flags after subcommand
sightjack --dry-run scan         # flags before subcommand
sightjack -n scan                # short alias
sightjack --lang=ja run          # --flag=value form
```

```bash
# Scan only (classify + deep-scan, no interactive loop)
sightjack scan

# Full interactive loop (scan + wave approval + apply)
sightjack run

# Display last scan results
sightjack show

# Dry run (generate prompts without executing Claude)
sightjack scan -n
sightjack run --dry-run

# Japanese prompts
sightjack run -l ja

# Custom config path
sightjack run -c .siren/config.yaml

# Auto-approve convergence gate (CI mode)
sightjack run --auto-approve

# Custom notification command
sightjack run --notify-cmd 'echo {title}: {message}'

# Custom approval command (exit 0 = approve)
sightjack run --approve-cmd 'my-approval-tool {message}'

# Verbose logging
sightjack run -v

# Scan a different repository
sightjack scan /path/to/repo

# Version info
sightjack version
sightjack version -j             # JSON output

# Check for updates
sightjack update -C              # check only
sightjack update                 # check and install

# Archive pruning
sightjack archive-prune -d 14   # 14-day retention (dry-run)
sightjack archive-prune -x      # execute deletion
```

### Unix pipeline

```bash
# Full pipeline: scan → select wave → discuss → apply
sightjack scan --json | sightjack waves | sightjack select | sightjack apply

# Generate ADR from discussion
sightjack scan --json | sightjack waves | sightjack select | sightjack discuss | sightjack adr > docs/adr/0005-foo.md

# Preview scan results
sightjack scan --json | sightjack show

# Save intermediate results
sightjack scan --json | tee scan.json | sightjack waves | tee plan.json | sightjack select > wave.json

# Generate follow-up waves after apply
cat wave.json | sightjack apply | sightjack nextgen
```

## Options

### Global flags (all subcommands)

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--config` | `-c` | `.siren/config.yaml` | Config file path |
| `--lang` | `-l` | config (`ja`) | Language override (`en` / `ja`) |
| `--verbose` | `-v` | `false` | Verbose logging |
| `--output` | `-o` | `text` | Output format: `text` or `json` |
| `--dry-run` | `-n` | `false` | Generate prompts without executing Claude |

### Subcommand flags

| Subcommand | Flag | Short | Default | Description |
|------------|------|-------|---------|-------------|
| `run` | `--notify-cmd` | | `""` | Notification command ({title}, {message} placeholders) |
| `run` | `--approve-cmd` | | `""` | Approval command ({message} placeholder, exit 0 = approve) |
| `run` | `--auto-approve` | | `false` | Skip approval gate for convergence D-Mail |
| `scan` | `--json` | `-j` | `false` | Output structured JSON |
| `version` | `--json` | `-j` | `false` | Output version info as JSON |
| `update` | `--check` | `-C` | `false` | Check for updates without installing |
| `archive-prune` | `--days` | `-d` | `30` | Retention days |
| `archive-prune` | `--execute` | `-x` | `false` | Execute deletion (default: dry-run) |

## Configuration

```yaml
# .siren/config.yaml
linear:
  team: "ENG"            # Linear team key
  project: "My Project"  # Linear project name
  cycle: ""              # Optional: filter by cycle

scan:
  chunk_size: 20         # Issues per scan chunk
  max_concurrency: 3     # Parallel scan workers

claude:
  command: "claude"      # Claude CLI command
  model: "opus"          # Model override (default: "opus")
  timeout_sec: 300       # Per-invocation timeout

scribe:
  enabled: true          # ADR generation via Scribe agent

strictness:
  default: "fog"         # Default strictness (fog/alert/lockdown)
  overrides:             # Per-label/cluster strictness (strictest wins)
    security: "lockdown"
    spike: "fog"

retry:
  max_attempts: 3        # Retry on Claude failures
  base_delay_sec: 2      # Base backoff between retries

labels:
  enabled: true          # Auto-label ready issues in Linear
  prefix: "sightjack"    # Label prefix (default: "sightjack")
  ready_label: "sightjack:ready"  # Ready-for-execution label name

gate:
  notify_cmd: ""         # Custom notification command ({title}, {message} placeholders)
  approve_cmd: ""        # Custom approval command ({message} placeholder, exit 0 = approve)
  auto_approve: false    # Skip approval gate for convergence D-Mail

dod_templates:           # Custom DoD templates by issue type
  api_endpoint:
    must:
      - "Error responses (4xx/5xx) defined"
      - "Auth/authz requirements specified"
    should:
      - "Rate limiting documented"

lang: "ja"               # Language (en/ja)
```

## Tracing (OpenTelemetry)

Sightjack instruments key operations (scan, wave generation, architect discussion, etc.) with OpenTelemetry spans and events. Tracing is off by default (noop tracer) and activates when `OTEL_EXPORTER_OTLP_ENDPOINT` is set.

```bash
# Start Jaeger (all-in-one trace viewer)
just jaeger

# Run with tracing enabled
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 sightjack run

# View traces at http://localhost:16686

# Stop Jaeger
just jaeger-down
```

## Development

```bash
# Task runner (just)
just build          # Build binary with version info (ldflags)
just install        # Build and install to /usr/local/bin
just test           # Run all tests
just test-v         # Verbose test output
just test-race      # Tests with race detector
just cover          # Coverage report
just cover-html     # Open coverage in browser
just fmt            # Format code (gofmt)
just vet            # Run go vet
just semgrep        # Run semgrep rules (ERROR severity only)
just lint           # fmt check + vet + markdown lint
just lint-md        # Lint markdown files only
just check          # fmt + vet + test (pre-commit check)
just docgen         # Generate CLI Markdown docs (docs/cli/)
just clean          # Clean build artifacts
just prek-install   # Install prek hooks (pre-commit + pre-push)
just prek-run       # Run all prek hooks on all files
just jaeger         # Start Jaeger trace viewer (docker)
just jaeger-down    # Stop Jaeger
```

## File Structure

```
+-- cmd/sightjack/
|   +-- main.go                 CLI entry point (signal handling, NeedsDefaultScan)
+-- internal/cmd/               Cobra CLI commands
|   +-- root.go                 Root command, persistent flags, OTel hooks
|   +-- scan.go                 scan subcommand (classify + deep-scan)
|   +-- waves.go                waves subcommand (wave generation)
|   +-- select.go               select subcommand (interactive wave picker via tty)
|   +-- discuss.go              discuss subcommand (Architect agent via tty)
|   +-- apply.go                apply subcommand (wave apply to Linear)
|   +-- adr.go                  adr subcommand (ADR generation)
|   +-- nextgen.go              nextgen subcommand (follow-up wave generation)
|   +-- show.go                 show subcommand (human-readable display)
|   +-- run.go                  run subcommand (interactive session loop)
|   +-- init.go                 init subcommand (config scaffolding)
|   +-- doctor.go               doctor subcommand (environment check)
|   +-- status.go               status subcommand (machine-readable state)
|   +-- archive_prune.go        archive-prune subcommand
|   +-- clean.go                clean subcommand
|   +-- version.go              version subcommand
|   +-- update.go               update subcommand (self-update)
|   +-- default_scan.go         NeedsDefaultScan + ReorderArgs
|   +-- telemetry.go            initTracer (OTLP HTTP exporter)
+-- internal/usecase/           Use case layer (PolicyEngine + handlers)
|   +-- policy.go               PolicyEngine type
|   +-- policy_handlers.go      Event-driven policy handlers
|   +-- scan.go                 Scan use case orchestration
|   +-- session.go              Session use case orchestration
|   +-- show.go                 Show use case orchestration
+-- internal/session/           I/O orchestration layer
|   +-- scanner.go              Scanner Agent (classify + deep-scan)
|   +-- architect.go            Architect Agent (discussion)
|   +-- scribe.go               Scribe Agent (ADR generation)
|   +-- session.go              Session lifecycle (run, resume)
|   +-- wave.go                 Wave model + unlock evaluation
|   +-- wave_generator.go       Wave generation + nextgen
|   +-- handoff.go              Handoff to downstream tools
|   +-- navigator.go            Matrix Navigator rendering
|   +-- cli.go                  Interactive prompts (selection, approval)
|   +-- claude.go               Claude Code subprocess runner
|   +-- config.go               Config loader (.siren/config.yaml)
|   +-- dmail.go                D-Mail file I/O (inbox/outbox/archive)
|   +-- outbox_store.go         SQLiteOutboxStore (transactional outbox)
|   +-- gate.go                 Convergence gate
|   +-- approve.go              Approver (Stdin, Cmd, Auto)
|   +-- notify.go               Notifier (Local, Cmd, Nop)
|   +-- review.go               Review subprocess
|   +-- review_gate.go          ReviewGate (exit code detection)
|   +-- recorder.go             Event recorder
|   +-- state_io.go             State persistence I/O
|   +-- status.go               Status query
|   +-- store_factory.go        Factory for cmd→eventsource access
|   +-- archive.go              Archive pruning
|   +-- skills.go               SKILL.md management
|   +-- doctor.go               Health check logic
|   +-- json_normalize.go       JSON UTF-8 normalization
|   +-- preflight.go            Pre-flight checks
|   +-- shell_quote.go          Cross-platform shell quoting
|   +-- shell_{unix,windows}.go Platform-specific shell
|   +-- signal_{unix,windows}.go Platform-specific signal handling
+-- internal/domain/            Pure domain functions
|   +-- projection.go           Event projection
|   +-- scan.go                 Scan utilities
|   +-- wave.go                 Wave scheduling
+-- internal/eventsource/       Event store infrastructure
|   +-- store_file.go           FileEventStore (JSONL append-only)
|   +-- lifecycle.go            Event file expiry + pruning
|   +-- loader.go               Event loader
|   +-- recorder.go             Event recorder
|   +-- path.go                 EventsDir path helper
+-- internal/tools/docgen/      CLI doc generator
+-- doc.go                      Package declaration (root-zero: all code in internal/)
+-- tests/scenario/             Scenario tests (L1-L4, //go:build scenario)
+-- tests/e2e/                  Docker E2E tests (//go:build e2e)
+-- .semgrep/                   Semgrep rules (layer enforcement)
+-- docs/cli/                   Auto-generated CLI reference
+-- docker/                     Jaeger v2 for trace viewing
+-- templates/                  AI prompt templates ({en,ja})
    +-- skills/                 D-Mail SKILL.md templates
```

## What / Why / How

See [docs/conformance.md](docs/conformance.md) for the full conformance table (single source).

## Documentation

- [docs/](docs/README.md) — Full documentation index
- [docs/conformance.md](docs/conformance.md) — What/Why/How conformance table
- [docs/siren-directory.md](docs/siren-directory.md) — `.siren/` directory structure
- [docs/policies.md](docs/policies.md) — Event → Policy mapping
- [docs/otel-backends.md](docs/otel-backends.md) — OTel backend configuration
- [docs/adr/](docs/adr/README.md) — Architecture Decision Records

## Prerequisites

- Go 1.26+
- [just](https://just.systems/) task runner
- [Claude Code CLI](https://docs.anthropic.com/en/docs/claude-code)
- [Linear MCP Server](https://github.com/anthropics/model-context-protocol) configured for Claude
- [Docker](https://www.docker.com/) for tracing (Jaeger)

## License

See [LICENSE](LICENSE) for details.
