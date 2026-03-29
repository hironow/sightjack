## sightjack sessions list

List recorded coding sessions

```
sightjack sessions list [flags]
```

### Options

```
  -h, --help            help for list
      --limit int       Max results (default 20)
      --status string   Filter by status (running, completed, failed, abandoned)
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

* [sightjack sessions](sightjack_sessions.md)	 - Manage AI coding sessions

