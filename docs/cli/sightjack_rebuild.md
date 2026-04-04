## sightjack rebuild

Rebuild projections from event store

### Synopsis

Replays all events from .siren/events/ to regenerate materialized projection state from scratch.
Saves a snapshot after successful replay for faster future recovery.

If path is omitted, the current working directory is used.

```
sightjack rebuild [path] [flags]
```

### Examples

```
  # Rebuild projections for the current directory
  sightjack rebuild

  # Rebuild projections for a specific project
  sightjack rebuild /path/to/repo
```

### Options

```
  -h, --help   help for rebuild
```

### Options inherited from parent commands

```
  -c, --config string   Config file path (default ".siren/config.yaml")
  -n, --dry-run         Generate prompts without executing Claude
  -l, --lang string     Language override (ja/en)
      --linear          Use Linear MCP for issue tracking (default: wave-centric mode)
      --no-color        Disable colored output (respects NO_COLOR env)
  -o, --output string   Output format: text, json (default "text")
  -q, --quiet           Suppress all stderr output
  -v, --verbose         Verbose logging
```

### SEE ALSO

* [sightjack](sightjack.md)	 - SIREN-inspired issue architecture tool for Linear

