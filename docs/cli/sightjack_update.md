## sightjack update

Self-update sightjack to the latest release

### Synopsis

Self-update sightjack to the latest GitHub release.

Downloads the latest release, verifies the checksum, and replaces
the current binary. Use --check to only check for updates without
installing.

```
sightjack update [flags]
```

### Examples

```
  # Check for updates
  sightjack update --check

  # Update to the latest version
  sightjack update
```

### Options

```
  -C, --check   Check for updates without installing
  -h, --help    help for update
```

### Options inherited from parent commands

```
  -c, --config string   Config file path (default ".siren/config.yaml")
  -n, --dry-run         Generate prompts without executing Claude
  -l, --lang string     Language override (ja/en)
  -v, --verbose         Verbose logging
```

### SEE ALSO

* [sightjack](sightjack.md)	 - SIREN-inspired issue architecture tool for Linear
