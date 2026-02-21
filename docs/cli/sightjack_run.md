## sightjack run

Interactive wave approval and apply loop

### Synopsis

Run an interactive session with wave approval and apply loop.

Combines scan → waves → select → apply → nextgen in a single
interactive session. Supports resume from a previous session
if state is found in .siren/state.json.

```
sightjack run [path] [flags]
```

### Examples

```
  # Start a new interactive session
  sightjack run

  # Resume a previous session (auto-detected)
  sightjack run

  # Dry-run mode (generate prompts without executing)
  sightjack run --dry-run
```

### Options

```
  -h, --help   help for run
```

### Options inherited from parent commands

```
  -c, --config string   Config file path (default ".siren/config.yaml")
      --dry-run         Generate prompts without executing Claude
  -l, --lang string     Language override (ja/en)
  -v, --verbose         Verbose logging
```

### SEE ALSO

* [sightjack](sightjack.md)	 - SIREN-inspired issue architecture tool for Linear
