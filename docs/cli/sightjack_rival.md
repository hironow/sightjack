## sightjack rival

Rival Contract v1.1 utilities

### Synopsis

Rival Contract v1.1 utilities.

Use 'rival export reasons' to project a Rival Contract v1 specification
into the OpenSPDD REASONS Canvas markdown shape.

### Examples

```
  # Project a stand-alone D-Mail file to REASONS Canvas markdown
  sightjack rival export reasons --input ./spec-auth_aaaaaaaa.md

  # Project the current revision for a wave to a file (JSON)
  sightjack rival export reasons --wave wave-x --format json --output canvas.json
```

### Options

```
  -h, --help   help for rival
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
* [sightjack rival export](sightjack_rival_export.md)	 - Export a Rival Contract to an interop format

