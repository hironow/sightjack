## sightjack doctor

Check environment and tool availability

### Synopsis

Check environment health and tool availability.

Verifies that the sightjack config is valid, required tools
(claude, git) are installed, and the Linear MCP connection
is working. Reports pass/fail/skip for each check.

```
sightjack doctor [path] [flags]
```

### Examples

```
  # Run environment check
  sightjack doctor

  # Check a specific project directory
  sightjack doctor /path/to/project
```

### Options

```
  -h, --help   help for doctor
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
