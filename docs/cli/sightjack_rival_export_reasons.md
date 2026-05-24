## sightjack rival export reasons

Export a Rival Contract v1 spec as OpenSPDD REASONS Canvas

### Synopsis

Project a Rival Contract v1 specification into OpenSPDD REASONS Canvas.

Reads either a stand-alone D-Mail file (--input) or the current revision
for a wave id resolved from the local archive (--wave). The two modes are
mutually exclusive.

Output is markdown by default; --format json emits a deterministic JSON
shape. The mapping is documented at:
  refs/plans/2026-05-03-rival-contract-v1-1-extensions.md §"Mapping"

```
sightjack rival export reasons [flags]
```

### Examples

```
  # Export a stand-alone D-Mail file to stdout
  sightjack rival export reasons --input ./spec-auth_aaaaaaaa.md

  # Export the current revision for a wave to a file
  sightjack rival export reasons --wave wave-auth-expiry --output canvas.md

  # JSON output for downstream tools
  sightjack rival export reasons --input ./spec.md --format json

  # Allow best-effort export under same-revision conflict
  sightjack rival export reasons --wave wave-x --allow-conflict
```

### Options

```
      --allow-conflict    Allow best-effort export under same-revision conflict (warns on stderr)
      --base-dir string   Project base directory for --wave archive resolution (default: cwd)
      --format string     Output format: markdown|json (default "markdown")
  -h, --help              help for reasons
      --input string      Path to a Rival Contract v1 specification D-Mail (.md)
      --output string     Output file path (default: stdout)
      --wave string       Wave id to resolve from the local archive (mutually exclusive with --input)
```

### Options inherited from parent commands

```
  -c, --config string   Config file path (default ".siren/config.yaml")
  -n, --dry-run         Generate prompts without executing Claude
  -l, --lang string     Language override (ja/en)
      --no-color        Disable colored output (respects NO_COLOR env)
  -q, --quiet           Suppress all stderr output
  -v, --verbose         Verbose logging
```

### SEE ALSO

* [sightjack rival export](sightjack_rival_export.md)	 - Export a Rival Contract to an interop format

