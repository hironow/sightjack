# `.siren/` Directory Structure

Sightjack manages all runtime state under `{baseDir}/.siren/`.
This document describes what each directory/file does, who creates it, and how it flows through the session lifecycle.

## Directory Tree

```
.siren/
  .gitignore              # auto-managed by WriteGitIgnore / EnsureScanDir
  config.yaml             # project-scoped settings (Linear team/project, strictness, lang)
  state.json              # current session state (waves, completeness, ADR count)
  skills/
    dmail-sendable/
      SKILL.md            # agent skill manifest (phonewave discovery)
    dmail-readable/
      SKILL.md
  inbox/                  # incoming d-mails (feedback from downstream)
    *.md
  outbox/                 # outgoing d-mails (specifications, reports)
    *.md
  archive/                # processed d-mails (permanent record)
    *.md
  .run/                   # ephemeral runtime data (per-session)
    {sessionID}/
      scan_result.json
      classify.json
      cluster_*.json
      wave_*.json
      architect_*.json
      apply_*.json
      scribe_*.json
      nextgen_*.json
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
state.json
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
| `state.json` | Ignored | Session-specific mutable state |
| `.run/` | Ignored | Ephemeral scan cache and Claude subprocess outputs |
| `inbox/` | Ignored | Transient; consumed and archived by MonitorInbox |
| `outbox/` | Ignored | Transient; courier (phonewave) picks up and delivers |
| `docs/adr/` | Tracked | Immutable decision records |

## Session Scan Cache: `.run/{sessionID}/`

Each session creates a unique directory under `.run/` containing all Claude subprocess outputs. Session ID format: `session-{unixmilli}-{pid}`.

| File Pattern | Claude Pass | Created By | Purpose |
|---|---|---|---|
| `scan_result.json` | — | `WriteScanResult()` | Cached scan results for session resume |
| `classify.json` | Pass 1 | Claude subprocess | Cluster classification and issue grouping |
| `cluster_{NN}_{name}_c{NN}.json` | Pass 2 | Claude subprocess | Deep scan results per cluster (chunked) |
| `wave_{NN}_{name}.json` | Pass 3 | Claude subprocess | Generated waves per cluster |
| `architect_{name}_{waveID}.json` | — | `RunArchitectDiscuss()` | Architect discussion response |
| `apply_{name}_{waveID}.json` | — | `RunWaveApply()` | Wave apply results (applied count, errors, ripples) |
| `scribe_{name}_{waveID}.json` | — | `RunScribeADR()` | Scribe ADR generation response |
| `nextgen_{name}_{waveID}.json` | — | `GenerateNextWaves()` | Follow-up waves after completion |
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
     |                      | -> LogInboxFeedbackAsync()   |
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
- **specification/report** -> **archive/** first, then **outbox/** (archive-first write order)
- D-mail format: YAML frontmatter (`name`, `kind`, `description`, `issues`, `severity`, `metadata`) + Markdown body
- D-mail kinds: `specification`, `report`, `feedback`
- Filename pattern: `{prefix}-{sanitized-wavekey}.md` (e.g., `spec-auth-w1.md`, `report-api-w2.md`)

## File Creators

| File | Created By | When |
|------|-----------|------|
| `.siren/` dirs | `EnsureScanDir()` | Session startup |
| `.gitignore` | `WriteGitIgnore()` | Session startup (via `EnsureScanDir`, idempotent) |
| `config.yaml` | `runInit()` | `sightjack init` command |
| `state.json` | `WriteState()` | After each wave select/approve/apply cycle |
| `skills/*/SKILL.md` | `InstallSkills()` | `sightjack init` command (overwrites existing) |
| `inbox/*.md` | External tool (phonewave) | Before/during session |
| `outbox/*.md` | `ComposeDMail()` | After wave approval or completion |
| `archive/*.md` | `ComposeDMail()` + `ReceiveDMail()` | After wave approval/completion or feedback receipt |
| `.run/{sessionID}/*` | Claude subprocess + `WriteScanResult()` | During scan/discuss/apply/scribe/nextgen |
| `docs/adr/NNNN-*.md` | `RunScribeADR()` | After architect modifies a wave (Scribe agent) |
| `.doctor_probe` | `checkStateDir()` | `sightjack doctor` (temporary, immediately removed) |
