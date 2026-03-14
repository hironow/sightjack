## sightjack archive-prune

Prune expired d-mails and event files

### Synopsis

Prune expired d-mails from the archive directory and
expired event files from the events directory.

Lists archived d-mail files and event files older than the retention threshold.
By default, runs in dry-run mode showing what would be deleted.
Pass --execute to actually remove the files.

```
sightjack archive-prune [path] [flags]
```

### Examples

```
  # Dry-run: list expired files (default 30 days)
  sightjack archive-prune

  # Delete expired files
  sightjack archive-prune --execute

  # JSON output for scripting
  sightjack archive-prune -o json

  # Custom retention period
  sightjack archive-prune --days 7 --execute

  # Rebuild archive index from existing files
  sightjack archive-prune --rebuild-index
```

### Options

```
  -d, --days int       Retention days for archive-prune (default 30)
  -x, --execute        Execute archive pruning (default: dry-run)
  -h, --help           help for archive-prune
      --rebuild-index  Rebuild archive index from existing files without pruning
  -y, --yes            Skip confirmation prompt
```

### Options inherited from parent commands

```
  -c, --config string   Config file path (default ".siren/config.yaml")
  -n, --dry-run         Generate prompts without executing Claude
  -l, --lang string     Language override (ja/en)
      --no-color        Disable colored output (respects NO_COLOR env)
  -o, --output string   Output format: text, json (default "text")
  -v, --verbose         Verbose logging
```

### SEE ALSO

* [sightjack](sightjack.md)  - SIREN-inspired issue architecture tool for Linear
