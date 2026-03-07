## sightjack adr

Generate ADR Markdown from stdin DiscussResult

### Synopsis

Generate an Architecture Decision Record from a DiscussResult on stdin.

Reads a DiscussResult JSON (from 'discuss') and renders it as
Markdown in the standard ADR format with auto-numbered filename.
Output is written to stdout for redirection to docs/adr/.

```
sightjack adr [path] [flags]
```

### Examples

```
  # Full pipeline: discuss → adr → file
  sightjack discuss | sightjack adr > docs/adr/0042-my-decision.md

  # Generate ADR from a saved discussion
  cat discussion.json | sightjack adr
```

### Options

```
  -h, --help   help for adr
```

### Options inherited from parent commands

```
  -c, --config string   Config file path (default ".siren/config.yaml")
  -n, --dry-run         Generate prompts without executing Claude
  -l, --lang string     Language override (ja/en)
  -o, --output string   Output format: text, json (default "text")
  -v, --verbose         Verbose logging
```

### SEE ALSO

* [sightjack](sightjack.md)	 - SIREN-inspired issue architecture tool for Linear

