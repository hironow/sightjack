## sightjack apply

Apply a wave to Linear from stdin Wave JSON

### Synopsis

Apply a wave to Linear issues from stdin Wave JSON.

Reads a Wave JSON (from 'select') and executes the wave plan against
Linear via Claude MCP tools. Outputs an ApplyResult JSON with updated
completeness, suitable for piping into 'nextgen' for follow-up wave
generation.

```
sightjack apply [path] [flags]
```

### Examples

```
  # Apply a selected wave and generate follow-ups
  sightjack select | sightjack apply | sightjack nextgen

  # Apply with dry-run
  sightjack select | sightjack apply --dry-run
```

### Options

```
  -h, --help   help for apply
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

