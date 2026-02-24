## sightjack show

Display last scan results

### Synopsis

Display scan results or wave plans.

When stdin is a pipe, auto-detects ScanResult or WavePlan JSON
and renders the appropriate navigator view. When run without pipe,
replays events from .siren/events/ and displays the matrix navigator.

```
sightjack show [path] [flags]
```

### Examples

```
  # Show from saved state
  sightjack show

  # Pipe a scan result
  sightjack scan --json | sightjack show

  # Pipe a wave plan
  sightjack scan --json | sightjack waves | sightjack show
```

### Options

```
  -h, --help   help for show
```

### Options inherited from parent commands

```
  -c, --config string   Config file path (default ".siren/config.yaml")
  -n, --dry-run         Generate prompts without executing Claude
  -l, --lang string     Language override (ja/en)
  -v, --verbose         Verbose logging
```

### SEE ALSO

* [sightjack](sightjack.md)	 - SIREN-inspired issue architecture tool for Linear
