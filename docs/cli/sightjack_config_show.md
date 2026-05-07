## sightjack config show

Display effective configuration

### Synopsis

Display the effective configuration after applying defaults and clamping.

```
sightjack config show [path] [flags]
```

### Options

```
  -h, --help   help for show
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

* [sightjack config](sightjack_config.md)	 - View or update sightjack configuration

