## sightjack

SIREN-inspired issue architecture tool for Linear

### Synopsis

sightjack — SIREN-inspired issue architecture tool for Linear

Classify, wave-plan, discuss, and apply changes to Linear issues.
Running without a subcommand defaults to 'scan'.
Use NeedsDefaultScan() to preprocess args before Execute.

### Options

```
  -c, --config string   Config file path (default ".siren/config.yaml")
  -n, --dry-run         Generate prompts without executing Claude
  -h, --help            help for sightjack
  -l, --lang string     Language override (ja/en)
      --linear          Use Linear MCP for issue tracking (default: wave-centric mode)
      --no-color        Disable colored output (respects NO_COLOR env)
  -o, --output string   Output format: text, json (default "text")
  -q, --quiet           Suppress all stderr output
  -v, --verbose         Verbose logging
```

### SEE ALSO

* [sightjack adr](sightjack_adr.md)	 - Generate ADR Markdown from stdin DiscussResult
* [sightjack apply](sightjack_apply.md)	 - Apply a wave from stdin Wave JSON
* [sightjack archive-prune](sightjack_archive-prune.md)	 - Prune expired d-mails and event files
* [sightjack clean](sightjack_clean.md)	 - Remove state directory (.siren/)
* [sightjack config](sightjack_config.md)	 - View or update sightjack configuration
* [sightjack dead-letters](sightjack_dead-letters.md)	 - Manage dead-lettered outbox items
* [sightjack discuss](sightjack_discuss.md)	 - Architect discussion from stdin Wave JSON
* [sightjack doctor](sightjack_doctor.md)	 - Check environment and tool availability
* [sightjack init](sightjack_init.md)	 - Create .siren/config.yaml
* [sightjack mcp-config](sightjack_mcp-config.md)	 - Manage MCP configuration for Claude subprocess isolation
* [sightjack nextgen](sightjack_nextgen.md)	 - Generate follow-up waves from stdin ApplyResult
* [sightjack rebuild](sightjack_rebuild.md)	 - Rebuild projections from event store
* [sightjack run](sightjack_run.md)	 - Interactive wave approval and apply loop
* [sightjack scan](sightjack_scan.md)	 - Classify and deep-scan issues
* [sightjack select](sightjack_select.md)	 - Interactively pick a wave from stdin WavePlan
* [sightjack sessions](sightjack_sessions.md)	 - Manage AI coding sessions
* [sightjack show](sightjack_show.md)	 - Display last scan results
* [sightjack status](sightjack_status.md)	 - Show sightjack operational status
* [sightjack update](sightjack_update.md)	 - Self-update sightjack to the latest release
* [sightjack version](sightjack_version.md)	 - Print version, commit, and build information
* [sightjack waves](sightjack_waves.md)	 - Generate waves from stdin ScanResult JSON

