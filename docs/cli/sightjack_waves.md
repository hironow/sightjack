## sightjack waves

Generate waves from stdin ScanResult JSON

### Synopsis

Generate wave plans from a ScanResult JSON on stdin.

Reads a ScanResult (from 'scan --json') and produces a WavePlan
containing prioritized waves for each cluster. Output is JSON suitable
for piping into 'select' or 'show'.

```
sightjack waves [path] [flags]
```

### Examples

```
  # Full pipe workflow
  sightjack scan --json | sightjack waves | sightjack show

  # Generate waves and save to file
  sightjack scan --json | sightjack waves > plan.json
```

### Options

```
  -h, --help   help for waves
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

