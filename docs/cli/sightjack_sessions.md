## sightjack sessions

Manage AI coding sessions

### Synopsis

Manage AI coding session records. Sessions are tracked in SQLite
and can be listed, filtered, and re-entered interactively.

### Examples

```
  sightjack sessions list
  sightjack sessions list --status completed --limit 5
  sightjack sessions enter <session-record-id>
  sightjack sessions enter --provider-id <claude-session-id>
```

### Options

```
  -h, --help   help for sessions
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
* [sightjack sessions enter](sightjack_sessions_enter.md)	 - Re-enter an AI coding session interactively
* [sightjack sessions list](sightjack_sessions_list.md)	 - List recorded coding sessions

