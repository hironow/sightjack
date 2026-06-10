# sightjack Claude Code integration

The `/sightjack-scan` entry skill moved into the embedded templates at
`internal/platform/templates/claude-skills/sightjack-scan/SKILL.md`
(single source of truth). `sightjack init` materializes it into the
target project's `.claude/skills/` so a bare `claude` session
auto-discovers it, and `sightjack mcp-config generate` upserts the
project-root `.mcp.json` (merge-aware) so the MCP server auto-attaches
(refs issue 0032, decision D5).

This directory is kept as a pointer; no plugin manifest machinery is
used.
