# Sightjack

**An MCP server + data plane for SIREN-inspired issue architecture: it serves the scan/wave read models from the session's scan dir and persists strictness config.**

Following the jun15 MCP pivot, LLM ownership moved to a human-initiated [Claude Code](https://docs.anthropic.com/en/docs/claude-code) session. Sightjack the Go CLI is now a pure data plane: it serves scan results and wave plans over MCP, persists scan strictness atomically, and provides the supporting data-plane commands. The headless designer pipeline — classify/deep-scan, wave generation, the interactive approve loop, the Architect/Scribe discuss + ADR steps, and D-Mail composition — has been retired. The scan/wave workflow now fires from the claude-code session via the `/sightjack-scan` skill and the sightjack MCP tools (see `plugins/sightjack/skills/sightjack-scan/SKILL.md`).

```bash
sightjack mcp
```

`sightjack mcp` starts the MCP server. Its tools expose:

1. `sightjack.ping` — health check
2. `sightjack.next_wave` — the first `available` wave from the session's scan dir
3. `sightjack.get_scan_result` — aggregated cluster info for a session
4. `sightjack.update_strictness` — atomically persist scan strictness to `.siren/config.yaml`

The claude-code session reads these read models, plans waves, and writes spec D-Mails to `outbox/` itself (via the skill workflow) — sightjack no longer drives the LLM or composes D-Mails.

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

Strictness uses a 3-layer resolution where strictness can only go up:

1. **Default** -- project-wide baseline from `strictness.default` config
2. **Estimated** -- auto-generated per-cluster by the LLM during deep scan, persisted to `strictness.estimated` in config
3. **Manual (Overrides)** -- explicit per-label/cluster overrides from `strictness.overrides` config (always wins if stronger)

Resolution: `max(default, estimated, overrides)` per cluster/label key.

- Affects wave generation: strict mode adds final consistency waves

### Wave Dynamic Evolution (Nextgen)

After each wave completion, the AI generates follow-up waves based on what was learned. Like SIREN's scenarios that only appear after certain conditions are met.

```
[Wave Complete] Auth W1: Dependency Ordering
  -> New wave generated: Auth W2: "JWT vs Session ADR" (emerged from W1 analysis)
  -> API W2 prerequisites updated (now depends on Auth W2)
```

### Wave Complexity Sorting

Waves are sorted by ascending complexity score before presentation. The score is computed as `len(Actions) + 0.5 * len(Prerequisites)`. Simpler waves surface first, encouraging incremental progress. The sort is stable so waves with equal complexity retain their original order.

### Dependency Cycle Detection

`DetectWaveCycles` performs DFS-based cycle detection on the wave prerequisite graph. If a cycle is found, wave generation fails with a descriptive error showing the cycle path. Self-referencing prerequisites are stripped by `RemoveSelfReferences` before cycle detection runs.

### Error Fingerprinting and Stall Escalation

Errors during wave apply are fingerprinted (`ErrorFingerprint`) and classified as structural or transient (`ClassifyError`). When the same structural error repeats above a threshold (`DetectRepeatedPattern`), the wave is marked "stalled" and a `stall-escalation` D-Mail is sent with severity "high".

### Error Budget (Circuit Breaker)

`ApplyErrorBudget` implements a circuit-breaker for wave apply operations. It tracks consecutive failures and trips the circuit when the configured threshold is reached. A single success resets the counter. When tripped, the session skips further apply attempts for graceful degradation.

### DoD Coverage Report

`BuildDoDCoverageReport` matches each cluster against configured DoD templates and reports which clusters are uncovered. This surfaces clusters that lack structured Definition of Done templates, prompting the user to add coverage.

### Scan Recovery Report

`BuildScanRecoveryReport` compares the full cluster list against wave generation successes to produce a `ScanRecoveryReport` with per-cluster outcomes, succeeded count, and failed count. This enables partial-result presentation when some clusters fail wave generation.

### ADR Lifecycle

- `SupersedeADR` patches an existing ADR file's `**Status:**` line to "Superseded by [new-id]"
- `FormatADRConflictSection` appends a `## Conflicts` section to scribe output when the generated ADR conflicts with existing ADRs

## D-Mail & Convergence Gate

Sightjack communicates with downstream tools (amadeus, paintress) via D-Mail, a file-based message protocol. D-Mail kinds:

| Kind | Direction | Description |
|------|-----------|-------------|
| `specification` | outbound | Wave specification sent to downstream tools |
| `report` | outbound | Wave completion report |
| `stall-escalation` | outbound | Wave stalled due to repeated structural errors (severity: high) |
| `design-feedback` | inbound | Design feedback from downstream tools (injected into nextgen prompts) |
| `implementation-feedback` | inbound | Implementation feedback from paintress (injected into nextgen prompts) |
| `convergence` | inbound | Convergence signal — requires user approval before session proceeds |

> Note: the headless wave loop, convergence gate, and D-Mail waiting phase that previously ran inside the retired `run`/`scan` pipeline have been retired with the jun15 MCP pivot. Spec/report/feedback D-Mails are now composed by the claude-code session via the `/sightjack-scan` skill, not the Go CLI. The kinds above document the D-Mail protocol the skill emits/consumes; sightjack the data plane only serves the scan/wave read models over MCP.

SKILL.md files in `.siren/skills/` declare produces/consumes routing for phonewave discovery using Agent Skills spec format with `dmail-schema-version: "1"`.

## Architecture

### Pipe Architecture

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
| Scribe | ADR generation from design decisions (auto-discuss in auto-approve mode) | Archive collection |
| (Handoff) | Ready-issue labeling for downstream tools | Next expedition |

## Scope

**What Sightjack does:**

- Scan Linear issues and detect missing DoD, hidden dependencies, and technical debt revival
- Generate wave-by-wave execution plans with prerequisite chains
- Guide interactive approval and apply approved changes to Linear via MCP
- Generate ADRs from design discussions (Scribe agent)
- Send specification/report D-Mails to downstream tools

**What Sightjack does NOT do:**

- Implement code changes (paintress handles implementation)
- Verify post-merge integrity (amadeus handles verification)
- Deliver D-Mails (phonewave handles routing)
- Modify source code or git repositories directly

## Setup

```bash
# Build and install
just install

# Initialize project config (Linear team key, etc.)
sightjack init

# Generate Claude subprocess isolation settings
sightjack mcp-config generate

# Re-initialize (upgrade SKILL.md, regenerate config)
sightjack init --force

# Check environment (config, tools, skills, event store, context-budget with per-item diagnostics, Docker)
sightjack doctor

# Start the MCP server (data plane for the claude-code scan session) — .siren/ is created automatically
sightjack mcp
```

Sightjack creates `.siren/` and all state/run files automatically at runtime. The `insights/` subdirectory is git-tracked and accumulates semantic knowledge (Shibito warnings, strictness estimates) across sessions.

## Subcommands

| Command | Description |
|---------|-------------|
| `mcp` | Start the MCP server (data plane: ping / next_wave / get_scan_result / update_strictness) |
| `sessions` | Manage coding sessions (`sessions enter` / `sessions list`) |
| `show` | Display last scan results from the event store |
| `init` | Initialize `.siren/config.yaml` |
| `doctor` | Check environment health |
| `config show` / `config set` | View or update configuration |
| `status` | Show operational status |
| `adr` | Generate ADR Markdown from a stdin `DiscussResult` |
| `clean` | Remove state directory |
| `archive-prune` | Remove expired scan archives |
| `rebuild` | Rebuild projections from the event store |
| `dead-letters` | Inspect / purge failed outbox deliveries |
| `rival` | Rival contract export utilities |
| `mcp-config generate` | Generate `.mcp.json` and `.claude/settings.json` for subprocess isolation |
| `version` | Print version info |
| `update` | Self-update to the latest release |

All commands accept an optional `[path]` argument (defaults to cwd). For flags, examples, and full reference per subcommand, see [docs/cli/](docs/cli/).

## Quick Start

```bash
sightjack init                    # set up .siren/
sightjack mcp-config generate     # Claude subprocess isolation settings
sightjack mcp                     # start the MCP data plane for the claude-code session
```

Then launch a claude-code session wired to the sightjack MCP server and invoke the `/sightjack-scan` skill; the session reads the scan/wave read models over MCP and drives the scan + wave planning itself.

## Configuration

```yaml
# .siren/config.yaml
tracker:
  team: "ENG"            # Linear team key
  project: "My Project"  # Linear project name
  cycle: ""              # Optional: filter by cycle

scan:
  chunk_size: 20         # Issues per scan chunk
  max_concurrency: 3     # Parallel scan workers

claude_cmd: "claude"     # Claude CLI command
model: "opus"            # Model override (default: "opus")
timeout_sec: 1980        # Per-invocation timeout (33 min)

scribe:
  enabled: true          # ADR generation via Scribe agent
  auto_discuss_rounds: 2 # Devil's Advocate rounds in auto-approve mode (0 = skip)

strictness:
  default: "fog"         # Default strictness (fog/alert/lockdown)
  overrides:             # Manual per-label/cluster strictness (always wins if stronger)
    security: "lockdown"
    spike: "fog"

computed:
  estimated_strictness:  # Auto-generated by LLM during scan (persisted after scan)
    auth: "alert"

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
  idle_timeout: 30m      # idle timeout — exit after no D-Mail activity (0 = 24h safety cap, <0 = disable)

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
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 sightjack mcp

# View traces at http://localhost:16686

# Stop Jaeger
just jaeger-down
```

## Development

All code lives in `internal/` (Go convention). The layer architecture includes `cmd`, `usecase`, `session`, `harness` (decision/validation/specification with `filter/`, `policy/`, `verifier/` sub-packages), `eventsource`, `platform`, and `domain`. See [docs/conformance.md](docs/conformance.md) for layer architecture, import rules, and directory responsibilities. Run `just --list` for available tasks.

## What / Why / How

See [docs/conformance.md](docs/conformance.md) for the full conformance table (single source).

## Documentation

- [docs/](docs/README.md) — Full documentation index
- [docs/conformance.md](docs/conformance.md) — What/Why/How conformance table
- [docs/siren-directory.md](docs/siren-directory.md) — `.siren/` directory structure
- [docs/policies.md](docs/policies.md) — Event → Policy mapping
- [docs/otel-backends.md](docs/otel-backends.md) — OTel backend configuration
- [docs/testing.md](docs/testing.md) — Test strategy and conventions
- [docs/adr/](docs/adr/README.md) — Architecture Decision Records
- [docs/shared-adr/](docs/shared-adr/README.md) — Cross-tool shared ADRs

## Prerequisites

- Go 1.26+
- [just](https://just.systems/) task runner
- [Claude Code CLI](https://docs.anthropic.com/en/docs/claude-code)
- [Linear MCP Server](https://github.com/anthropics/model-context-protocol) configured for Claude
- [Docker](https://www.docker.com/) for tracing (Jaeger)

## License

See [LICENSE](LICENSE) for details.
