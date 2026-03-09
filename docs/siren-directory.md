# `.siren/` Directory Structure

Sightjack manages all runtime state under `{baseDir}/.siren/`.
This document describes what each directory/file does, who creates it, and how it flows through the session lifecycle.

## Directory Tree

```
.siren/
  .gitignore              # auto-managed by WriteGitIgnore / EnsureScanDir
  config.yaml             # project-scoped settings (Linear team/project, strictness, lang)
  events/                 # append-only event logs (JSONL, one file per session)
    {sessionID}.jsonl     # immutable event stream for a session
  skills/
    dmail-sendable/
      SKILL.md            # agent skill manifest (produces: specification, report)
    dmail-readable/
      SKILL.md            # agent skill manifest (consumes: design-feedback, convergence)
  inbox/                  # incoming d-mails (feedback from downstream)
    *.md
  outbox/                 # outgoing d-mails (specifications, reports)
    *.md
  archive/                # processed d-mails (permanent record)
    *.md
  .run/                   # ephemeral runtime data (per-session)
    {sessionID}/
      scan_result.json    # cached ScanResult (WriteScanResult / scan --json)
      classify.json       # Claude Pass 1 output
      cluster_*.json      # Claude Pass 2 deep-scan per cluster
      wave_*.json         # Claude Pass 3 wave generation per cluster
      waves_result.json   # cached WavePlan (waves subcommand)
      architect_*.json    # Architect discussion response per wave
      discuss_result.json # cached DiscussResult (discuss subcommand)
      apply_*.json        # Wave apply response per wave
      apply_result.json   # cached ApplyResult (apply subcommand)
      scribe_*.json       # Scribe ADR generation per wave
      nextgen_*.json      # Follow-up wave response per wave
      nextgen_result.json # cached WavePlan (nextgen subcommand)
      *_prompt.md         # dry-run mode only
```

Additionally, sightjack creates files outside `.siren/`:

```
docs/
  adr/
    NNNN-*.md             # Architecture Decision Records (Scribe agent)
```

## Git Tracking Rules

`.siren/.gitignore` (auto-managed by `WriteGitIgnore`):

```
events/
.run/
inbox/
outbox/
```

| Path | Git Status | Reason |
|------|-----------|--------|
| `config.yaml` | Tracked | Project-level configuration |
| `skills/` | Tracked | Agent capability manifests for tool discovery |
| `archive/` | Tracked | Audit trail of all d-mail activity |
| `.gitignore` | Tracked | Self-managed ignore rules |
| `events/` | Ignored | Session-specific event logs (source of truth for state) |
| `.run/` | Ignored | Ephemeral scan cache and Claude subprocess outputs |
| `inbox/` | Ignored | Transient; consumed and archived by MonitorInbox |
| `outbox/` | Ignored | Transient; courier (phonewave) picks up and delivers |
| `docs/adr/` | Tracked | Immutable decision records |

## Event Sourcing

Session state is derived from append-only event logs stored in `.siren/events/{sessionID}.jsonl`.
Each line is a JSON-encoded event with `type`, `timestamp`, `session_id`, `sequence`, and `payload`.

State is reconstructed by replaying all events via `ProjectState()`. There is no snapshot file;
the event log is the single source of truth. See ADR 0008 for design rationale.

Key functions:

- `LoadState(store)` — replay events from a specific EventStore
- `LoadLatestState(baseDir)` — find newest `.jsonl` in events/, replay to get current state

## Session Scan Cache: `.run/{sessionID}/`

Each session creates a unique directory under `.run/` containing all Claude subprocess outputs. Session ID format: `session-{unixmilli}-{pid}`.

| File Pattern | Claude Pass | Created By | Purpose |
|---|---|---|---|
| `scan_result.json` | — | `WriteScanResult()` | Cached ScanResult for session resume |
| `classify.json` | Pass 1 | Claude subprocess | Cluster classification and issue grouping |
| `cluster_{NN}_{name}_c{NN}.json` | Pass 2 | Claude subprocess | Deep scan results per cluster (chunked) |
| `wave_{NN}_{name}.json` | Pass 3 | Claude subprocess | Generated waves per cluster |
| `waves_result.json` | — | `waves` subcommand | Cached aggregated WavePlan for pipe replay |
| `architect_{name}_{waveID}.json` | — | `RunArchitectDiscuss()` | Architect discussion response |
| `discuss_result.json` | — | `discuss` subcommand | Cached DiscussResult for pipe replay |
| `apply_{name}_{waveID}.json` | — | `RunWaveApply()` | Wave apply results (applied count, errors, ripples) |
| `apply_result.json` | — | `apply` subcommand | Cached ApplyResult for pipe replay |
| `scribe_{name}_{waveID}.json` | — | `RunScribeADR()` | Scribe ADR generation response |
| `nextgen_{name}_{waveID}.json` | — | `GenerateNextWaves()` | Follow-up waves after completion |
| `nextgen_result.json` | — | `nextgen` subcommand | Cached WavePlan for pipe replay |
| `*_prompt.md` | — | `RunClaudeDryRun()` | Prompt files saved in `--dry-run` mode |

All `{name}` values are sanitized via `sanitizeName()` (scanner.go) to prevent path traversal.

## D-Mail Lifecycle

```
[downstream tool]        sightjack                   [downstream tool]
     |                      |                              |
     | writes to inbox/     |                              |
     |--------------------->|                              |
     |                      | MonitorInbox() [fsnotify]    |
     |                      | receiveDMailIfNew()          |
     |                      | -> DrainInboxFeedback()      |
     |                      |                              |
     |                      | RunConvergenceGateWithRedrain|
     |                      |   FilterConvergence()        |
     |                      |   Notifier.Notify() [async]  |
     |                      |   Approver.RequestApproval() |
     |                      |   re-drain if new convergence|
     |                      |                              |
     |                      | -> CollectFeedback()         |
     |                      |    (accumulates for nextgen) |
     |                      |    (convergence -> notify)   |
     |                      |                              |
     |                      | (wave approved)              |
     |                      |   ComposeSpecification()     |
     |                      |   -> outbox/ + archive/      |
     |                      |                              |
     |                      | (wave completed)             |
     |                      |   ComposeReport()            |
     |                      |   -> outbox/ + archive/      |
     |                      |                              |
     |                      |              reads outbox/   |
     |                      |----------------------------->|
```

- **inbox/** -> fsnotify monitor -> **archive/** (consumed by `ReceiveDMail`)
- **convergence gate** -> `RunConvergenceGateWithRedrain()` runs notify + approve loop before session starts. Re-drains inbox after each approval to catch late-arriving convergence D-Mails.
- **specification/report** -> **archive/** first, then **outbox/** (archive-first write order)
- D-mail format: YAML frontmatter (`name`, `kind`, `description`, `dmail-schema-version`, `issues`, `severity`, `metadata`) + Markdown body
- D-mail kinds: `specification`, `report`, `design-feedback`, `convergence`
- Filename pattern: `{prefix}-{sanitized-wavekey}.md` (e.g., `spec-auth-w1.md`, `report-api-w2.md`)

## File Creators

| File | Created By | When |
|------|-----------|------|
| `.siren/` dirs | `EnsureScanDir()` | Session startup |
| `.gitignore` | `WriteGitIgnore()` | Session startup (via `EnsureScanDir`, idempotent) |
| `config.yaml` | `runInit()` | `sightjack init` command (use `--force` to overwrite) |
| `events/{sessionID}.jsonl` | `SessionRecorder.Record()` | Each state-changing action during a session |
| `skills/*/SKILL.md` | `InstallSkills()` | `sightjack init` command (overwrites existing) |
| `inbox/*.md` | External tool (phonewave) | Before/during session |
| `outbox/*.md` | `ComposeDMail()` | After wave approval or completion |
| `archive/*.md` | `ComposeDMail()` + `ReceiveDMail()` | After wave approval/completion or feedback receipt |
| `.run/{sessionID}/*` | Claude subprocess + `WriteScanResult()` | During scan/discuss/apply/scribe/nextgen |
| `docs/adr/NNNN-*.md` | `RunScribeADR()` | After architect modifies a wave (Scribe agent) |
| `.doctor_probe` | `checkStateDir()` | `sightjack doctor` (temporary, immediately removed) |
