## sightjack status

Show sightjack operational status

### Synopsis

Display operational status including scan history, wave statistics,
success rate, and pending d-mail counts.

Output goes to stdout by default (human-readable text).
Use -o json for machine-readable JSON output to stdout.

```
sightjack status [path] [flags]
```

### Examples

```
  # Show status for current directory
  sightjack status

  # Show status for a specific project
  sightjack status /path/to/project

  # JSON output for scripting
  sightjack status -o json
```

### Options

```
  -h, --help   help for status
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

* [sightjack](sightjack.md)  - SIREN-inspired issue architecture tool for Linear
