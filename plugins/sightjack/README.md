# sightjack claude code plugin (jun15 MCP pivot)

**Status:** Phase 2a in progress (= MCP server stub + skill skeleton).
Production target for the post-2026-06-15 architecture where claude
code interactive sessions own LLM inference and the sightjack Go CLI
exposes its data/control plane as an MCP server. Pattern referenced
from paintress Phase 1 (ADR 0017).

## Layout

```
plugins/sightjack/
├── README.md                          # this file
└── skills/
    └── sightjack-scan/SKILL.md        # /sightjack-scan slash command
```

Subsequent commits on `feat/jun15-mcp-pivot` add:

- `agents/scan.md` — long-running scan + wave generation agent (post-stub)
- `skills/check-reports/SKILL.md` — explicit D-Mail consume entry point
- `hooks/` — non-LLM hooks only (e.g. stderr-only inbox count notice)

## Loading the plugin

```bash
claude \
  --plugin-dir ./plugins/sightjack \
  --mcp-config '{"sightjack":{"command":"sightjack","args":["mcp"]}}'
```

The `--plugin-dir` flag registers the skills; the `--mcp-config` flag
attaches the sightjack MCP server (`sightjack mcp` subcommand) so the
skill's `mcp__sightjack__*` tools resolve.

## Phase 2a MVP scope

Only `/sightjack-scan` is wired. The slash command calls the sightjack
MCP server's stub tools (sightjack.ping, sightjack.next_wave,
sightjack.get_scan_result, sightjack.update_strictness) and surfaces
the stub contract to the human. Real domain wiring lands in subsequent
commits on `feat/jun15-mcp-pivot`.

## Distinct from `sightjack mcp-config`

`sightjack mcp-config` (legacy) manages the `.mcp.json` config consumed
by the embedded claude_adapter. `sightjack mcp` (this plugin) is the
**server** consumed by claude code itself. The two have different roles
and coexist during the pivot transition.
