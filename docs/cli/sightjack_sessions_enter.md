## sightjack sessions enter

Re-enter an AI coding session interactively

### Synopsis

Launches the provider CLI in interactive mode with --resume, preserving isolation flags.
Pass a session record ID or use --provider-id for direct provider session targeting.

```
sightjack sessions enter [session-record-id] [flags]
```

### Options

```
  -h, --help                 help for enter
      --path string          Repository root path
      --provider-id string   Resume by provider session ID directly
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

