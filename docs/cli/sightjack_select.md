## sightjack select

Interactively pick a wave from stdin WavePlan

### Synopsis

Interactively pick a wave from a WavePlan JSON on stdin.

Presents available waves and prompts for selection via /dev/tty.
Outputs the selected wave as JSON with remaining sibling context
for downstream commands (apply, discuss).

```
sightjack select [flags]
```

### Examples

```
  # Select a wave and apply it
  sightjack scan --json | sightjack waves | sightjack select | sightjack apply

  # Select a wave and start a discussion
  sightjack scan --json | sightjack waves | sightjack select | sightjack discuss
```

### Options

```
  -h, --help   help for select
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
