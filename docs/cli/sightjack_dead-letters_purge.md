## sightjack dead-letters purge

Remove dead-lettered outbox items

### Synopsis

Remove outbox items that have failed delivery 3+ times.

By default, runs in dry-run mode showing the count of dead-lettered items.
Pass --execute to actually remove them.

```
sightjack dead-letters purge [path] [flags]
```

### Examples

```
  # Dry-run: show dead letter count
  sightjack dead-letters purge

  # Remove dead-lettered items
  sightjack dead-letters purge --execute

  # Skip confirmation prompt
  sightjack dead-letters purge --execute --yes

  # JSON output for scripting
  sightjack dead-letters purge -o json
```

### Options

```
  -x, --execute   Execute purge (default: dry-run)
  -h, --help      help for purge
  -y, --yes       Skip confirmation prompt
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

* [sightjack dead-letters](sightjack_dead-letters.md)	 - Manage dead-lettered outbox items

