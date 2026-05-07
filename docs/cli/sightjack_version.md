## sightjack version

Print version, commit, and build information

### Synopsis

Print version, commit hash, build date, Go version, and OS/arch.

By default outputs a human-readable single line. Use --json
for structured output suitable for scripts and CI.

```
sightjack version [flags]
```

### Examples

```
  sightjack version
  sightjack version -j
```

### Options

```
  -h, --help   help for version
  -j, --json   Output version info as JSON
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

