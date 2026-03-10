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
      --no-color        Disable colored output (respects NO_COLOR env)
  -o, --output string   Output format: text, json (default "text")
  -v, --verbose         Verbose logging
```

### SEE ALSO

* [sightjack adr](sightjack_adr.md)	 - Generate ADR Markdown from stdin DiscussResult
* [sightjack apply](sightjack_apply.md)	 - Apply a wave to Linear from stdin Wave JSON
* [sightjack archive-prune](sightjack_archive-prune.md)	 - Prune expired d-mails and event files
* [sightjack clean](sightjack_clean.md)	 - Remove state directory (.siren/)
* [sightjack config](sightjack_config.md)	 - View or update sightjack configuration
* [sightjack discuss](sightjack_discuss.md)	 - Architect discussion from stdin Wave JSON
* [sightjack doctor](sightjack_doctor.md)	 - Check environment and tool availability
* [sightjack init](sightjack_init.md)	 - Create .siren/config.yaml
* [sightjack nextgen](sightjack_nextgen.md)	 - Generate follow-up waves from stdin ApplyResult
* [sightjack run](sightjack_run.md)	 - Interactive wave approval and apply loop
* [sightjack scan](sightjack_scan.md)	 - Classify and deep-scan Linear issues
* [sightjack select](sightjack_select.md)	 - Interactively pick a wave from stdin WavePlan
* [sightjack show](sightjack_show.md)	 - Display last scan results
* [sightjack status](sightjack_status.md)	 - Show sightjack operational status
* [sightjack update](sightjack_update.md)	 - Self-update sightjack to the latest release
* [sightjack version](sightjack_version.md)	 - Print version, commit, and build information
* [sightjack waves](sightjack_waves.md)	 - Generate waves from stdin ScanResult JSON

