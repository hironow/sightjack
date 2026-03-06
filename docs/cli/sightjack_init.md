## sightjack init

Create .siren/config.yaml

### Synopsis

Initialize a new sightjack project by creating .siren/config.yaml.

Use --team and --project flags for non-interactive mode, or omit
flags for interactive prompts. Also creates .gitignore, installs
d-mail skills, and sets up mail directories.

```
sightjack init [path] [flags]
```

### Examples

```
  # Non-interactive with flags
  sightjack init --team Engineering --project Hades

  # Initialize in a specific directory
  sightjack init --team Engineering --project Hades /path/to/project

  # Defaults only (no prompts)
  sightjack init /path/to/project
```

### Options

```
  -h, --help                  help for init
      --lang string           Language (ja/en) (default "ja")
      --otel-backend string   OTel backend: jaeger, weave
      --otel-entity string    Weave entity/team (required for weave)
      --otel-project string   Weave project (required for weave)
      --project string        Linear project name
      --strictness string     Strictness level (fog/alert/lockdown) (default "fog")
      --team string           Linear team name
```

### Options inherited from parent commands

```
  -c, --config string   Config file path (default ".siren/config.yaml")
  -n, --dry-run         Generate prompts without executing Claude
  -o, --output string   Output format: text, json (default "text")
  -v, --verbose         Verbose logging
```

### SEE ALSO

* [sightjack](sightjack.md)  - SIREN-inspired issue architecture tool for Linear
