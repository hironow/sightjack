## sightjack rival export

Export a Rival Contract to an interop format

### Synopsis

Export a Rival Contract v1 specification to a downstream tool's format.

Available targets:
  reasons   OpenSPDD REASONS Canvas (markdown or JSON).

### Examples

```
  # OpenSPDD REASONS Canvas (markdown)
  sightjack rival export reasons --input ./spec.md

  # OpenSPDD REASONS Canvas (JSON)
  sightjack rival export reasons --input ./spec.md --format json
```

### Options

```
  -h, --help   help for export
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

* [sightjack rival](sightjack_rival.md)  - Rival Contract v1.1 utilities
* [sightjack rival export reasons](sightjack_rival_export_reasons.md)  - Export a Rival Contract v1 spec as OpenSPDD REASONS Canvas
