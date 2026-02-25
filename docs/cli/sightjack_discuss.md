## sightjack discuss

Architect discussion from stdin Wave JSON

### Synopsis

Start an architect discussion about a wave from stdin.

Reads a Wave JSON (from 'select') and prompts for a discussion topic
via /dev/tty. Runs the architect agent to produce a DiscussResult
suitable for piping into 'adr' for ADR generation.

```
sightjack discuss [path] [flags]
```

### Examples

```
  # Discuss a selected wave and generate an ADR
  sightjack select | sightjack discuss | sightjack adr > docs/adr/NNNN.md

  # Discuss with a specific project directory
  sightjack select | sightjack discuss /path/to/project
```

### Options

```
  -h, --help   help for discuss
```

### Options inherited from parent commands

```
  -c, --config string   Config file path (default ".siren/config.yaml")
  -n, --dry-run         Generate prompts without executing Claude
  -l, --lang string     Language override (ja/en)
  -v, --verbose         Verbose logging
```

### SEE ALSO

* [sightjack](sightjack.md)  - SIREN-inspired issue architecture tool for Linear
