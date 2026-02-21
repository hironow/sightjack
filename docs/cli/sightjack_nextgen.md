## sightjack nextgen

Generate follow-up waves from stdin ApplyResult

### Synopsis

Generate follow-up waves from an ApplyResult on stdin.

Reads an ApplyResult JSON (from 'apply') and evaluates whether
additional waves are needed based on completeness thresholds.
If more waves are needed, calls the AI to generate them.
Outputs a WavePlan JSON suitable for piping back into 'show' or 'select'.

```
sightjack nextgen [path] [flags]
```

### Examples

```
  # Apply and generate follow-up waves
  sightjack apply | sightjack nextgen | sightjack show

  # Full cycle: select → apply → nextgen → select
  sightjack select | sightjack apply | sightjack nextgen | sightjack select
```

### Options

```
  -h, --help   help for nextgen
```

### Options inherited from parent commands

```
  -c, --config string   Config file path (default ".siren/config.yaml")
      --dry-run         Generate prompts without executing Claude
  -l, --lang string     Language override (ja/en)
  -v, --verbose         Verbose logging
```

### SEE ALSO

* [sightjack](sightjack.md)	 - SIREN-inspired issue architecture tool for Linear
