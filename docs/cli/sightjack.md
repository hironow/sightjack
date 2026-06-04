## sightjack

SIREN-inspired issue architecture MCP data plane

### Synopsis

sightjack — SIREN-inspired issue architecture MCP data plane

Serve scan/wave read models to a human-initiated Claude Code session via
the `sightjack mcp` stdio server + the /sightjack-scan skill (jun15 MCP
pivot). Use `sightjack sessions` to manage coding sessions and the
data-plane commands (show, status, rebuild) to inspect state.

### Options

```
  -c, --config string   Config file path (default ".siren/config.yaml")
  -n, --dry-run         Generate prompts without executing Claude
  -h, --help            help for sightjack
  -l, --lang string     Language override (ja/en)
      --no-color        Disable colored output (respects NO_COLOR env)
  -o, --output string   Output format: text, json (default "text")
  -q, --quiet           Suppress all stderr output
  -v, --verbose         Verbose logging
```

### SEE ALSO

* [sightjack adr](sightjack_adr.md)	 - Generate ADR Markdown from stdin DiscussResult
* [sightjack archive-prune](sightjack_archive-prune.md)	 - Prune expired d-mails and event files
* [sightjack clean](sightjack_clean.md)	 - Remove state directory (.siren/)
* [sightjack config](sightjack_config.md)	 - View or update sightjack configuration
* [sightjack dead-letters](sightjack_dead-letters.md)	 - Manage dead-lettered outbox items
* [sightjack doctor](sightjack_doctor.md)	 - Check environment and tool availability
* [sightjack init](sightjack_init.md)	 - Create .siren/config.yaml
* [sightjack mcp](sightjack_mcp.md)	 - Run sightjack as an MCP server over stdio (scan/wave data plane + strictness control)
* [sightjack mcp-config](sightjack_mcp-config.md)	 - Manage MCP wiring for Claude Code sessions
* [sightjack rebuild](sightjack_rebuild.md)	 - Rebuild projections from event store
* [sightjack rival](sightjack_rival.md)	 - Rival Contract v1.1 utilities
* [sightjack sessions](sightjack_sessions.md)	 - Manage AI coding sessions
* [sightjack show](sightjack_show.md)	 - Display last scan results
* [sightjack status](sightjack_status.md)	 - Show sightjack operational status
* [sightjack update](sightjack_update.md)	 - Self-update sightjack to the latest release
* [sightjack version](sightjack_version.md)	 - Print version, commit, and build information

