## sightjack scan

Classify and deep-scan Linear issues

### Synopsis

Classify and deep-scan Linear issues in the configured project.

Connects to the Linear API, fetches issues, and produces a ScanResult
with cluster classification, completeness scores, and shibito warnings.
Use --json to output structured JSON for piping into downstream commands.

```
sightjack scan [path] [flags]
```

### Examples

```
  # Interactive scan with navigator display
  sightjack scan

  # Pipe workflow: scan → waves → show
  sightjack scan --json | sightjack waves | sightjack show

  # Scan a specific project directory
  sightjack scan /path/to/project
```

### Options

```
  -h, --help   help for scan
  -j, --json   Output scan result as JSON
```

### Options inherited from parent commands

```
  -c, --config string   Config file path (default ".siren/config.yaml")
  -n, --dry-run         Generate prompts without executing Claude
  -l, --lang string     Language override (ja/en)
  -v, --verbose         Verbose logging
```

### SEE ALSO

* [sightjack](sightjack.md)  - SIREN-inspired issue architecture tool for Linear
