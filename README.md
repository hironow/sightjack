# sightjack

SIREN-inspired issue architecture tool for Linear — powered by Claude Code AI agents.

sightjack scans your Linear project, classifies issue clusters, generates execution waves, and guides you through an interactive approval workflow to bring order to complex backlogs.

## Prerequisites

- Go 1.25+
- [Claude Code CLI](https://docs.anthropic.com/en/docs/claude-code)
- [Linear MCP Server](https://github.com/anthropics/model-context-protocol) configured for Claude

## Install

```bash
go install github.com/hironow/sightjack/cmd/sightjack@latest
```

## Quick Start

Create a `sightjack.yaml` in your project root:

```yaml
linear:
  team: "ENG"
  project: "My Project"
lang: "en"
```

Then run:

```bash
# Scan and classify issues (default command)
sightjack scan

# Start an interactive session (scan + wave approval + apply)
sightjack session

# Display results from the last scan
sightjack show
```

## Commands

| Command   | Description                                          |
|-----------|------------------------------------------------------|
| `scan`    | Classify and deep-scan Linear issues (default)       |
| `session` | Interactive wave approval and apply session           |
| `show`    | Display last scan results                             |

## Flags

| Flag              | Short | Description                              |
|-------------------|-------|------------------------------------------|
| `--config`        | `-c`  | Config file path (default: sightjack.yaml) |
| `--lang`          | `-l`  | Language override (ja/en)                |
| `--verbose`       | `-v`  | Verbose logging                          |
| `--dry-run`       |       | Generate prompts without executing Claude |
| `--version`       |       | Show version                             |

## Configuration

`sightjack.yaml` reference:

```yaml
linear:
  team: "ENG"           # Linear team key
  project: "My Project" # Linear project name
  cycle: ""             # Optional: filter by cycle

scan:
  chunk_size: 50        # Issues per scan chunk
  max_concurrency: 3    # Parallel scan workers

claude:
  command: "claude"     # Claude CLI command
  model: ""             # Model override (optional)
  timeout_sec: 120      # Per-invocation timeout

scribe:
  enabled: true         # Enable ADR generation via Scribe agent

strictness:
  default: "normal"     # Default strictness level

labels:
  enabled: false        # Auto-label ready issues in Linear

lang: "en"              # Language (en/ja)
```

## Architecture

sightjack uses a 4-pass system:

```
+------------------+     +------------------+     +------------------+     +------------------+
|  Pass 1          |     |  Pass 2          |     |  Pass 3          |     |  Pass 4          |
|  Classify        | --> |  DeepScan        | --> |  WaveGenerate    | --> |  WaveApply       |
|  (cluster issues)|     |  (per-cluster)   |     |  (create waves)  |     |  (apply changes) |
+------------------+     +------------------+     +------------------+     +------------------+
```

Legend:
- Classify: Issues are grouped into thematic clusters
- DeepScan: Each cluster is analyzed for completeness and dependencies
- WaveGenerate: Ordered execution waves are created with prerequisites
- WaveApply: Approved waves are applied to Linear issues

State is persisted to `.siren/state.json` for session resume support.

See `docs/` for detailed architecture documentation.

## License

See [LICENSE](LICENSE) for details.
