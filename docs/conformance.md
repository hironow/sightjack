# What / Why / How Conformance

This is the single source of truth for sightjack's purpose, design rationale, and implementation approach.
Referenced from [README.md](../README.md) and [docs/README.md](README.md).

| Aspect | Description |
|--------|-------------|
| **What** | Interactive AI session that analyzes Linear issues for completeness, dependencies, and architectural gaps |
| **Why** | Bring issue completeness from ~30% to ~85% before autonomous execution begins |
| **How** | Claude MCP tools scan issues → cluster analysis → wave generation → interactive approval → apply to Linear |
| **Input** | Linear issues via Claude MCP tools, user approval via stdin |
| **Output** | Updated Linear issues, D-Mail reports to downstream tools |
| **Telemetry** | OTel spans: `sightjack.scan`, `claude.invoke` (with `claude.model`, `claude.timeout_sec`, `gen_ai.*`) |
| **External Systems** | Linear (via Claude MCP), Claude Code subprocess, OTel exporter (Jaeger/Weave) |

## Cross-Tool Conformance

All 4 tools (phonewave, sightjack, paintress, amadeus) maintain a What/Why/How conformance table in `docs/conformance.md` with the same structure. This prevents expression drift across README files.
