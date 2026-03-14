## sightjack clean

Remove state directory (.siren/)

### Synopsis

Delete the .siren/ directory to reset to a clean state. Use 'sightjack init' to reinitialize.

```
sightjack clean [path] [flags]
```

### Examples

```
  sightjack clean
  sightjack clean /path/to/project
  sightjack clean --yes
```

### Options

```
  -h, --help   help for clean
      --yes    Skip confirmation prompt
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

* [sightjack](sightjack.md)	 - SIREN-inspired issue architecture tool for Linear

