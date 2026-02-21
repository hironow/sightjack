## sightjack archive-prune

Prune expired d-mails from archive

### Synopsis

Prune expired d-mails from the archive directory.

Lists archived d-mail files older than the retention threshold.
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

  # Custom retention period
  sightjack archive-prune --days 7 --execute
```

### Options

```
  -d, --days int   Retention days for archive-prune (default 30)
  -x, --execute    Execute archive pruning (default: dry-run)
  -h, --help       help for archive-prune
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
