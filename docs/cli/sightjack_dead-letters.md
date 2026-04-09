## sightjack dead-letters

Manage dead-lettered outbox items

### Synopsis

Manage outbox items that have exceeded the maximum retry count
and are permanently stuck.

Use the purge subcommand to remove dead-lettered items.

### Examples

```
  # Show dead-letter count (dry-run)
  sightjack dead-letters purge

  # Remove dead-lettered items
  sightjack dead-letters purge --execute --yes
```

### Options

```
  -h, --help   help for dead-letters
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
* [sightjack dead-letters purge](sightjack_dead-letters_purge.md)	 - Remove dead-lettered outbox items

