## sightjack doctor

Check environment and tool availability

### Synopsis

Check environment health and tool availability.

Verifies that the sightjack config is valid, required tools
(claude, git) are installed, and the Linear MCP connection
is working. Each check reports one of four statuses:
OK (passed), FAIL (exit 1), SKIP (dependency missing),
WARN (advisory, exit 0).

The context-budget check estimates token consumption per category
(tools, skills, plugins, mcp, hooks) and marks the heaviest.
When the threshold (20,000 tokens) is exceeded, a category-specific
hint recommends adjusting .claude/settings.json.

```
sightjack doctor [path] [flags]
```

### Examples

```
  # Run environment check
  sightjack doctor

  # Check a specific project directory
  sightjack doctor /path/to/project

  # JSON output for scripting
  sightjack doctor -o json

  # Auto-fix repairable issues
  sightjack doctor --repair
```

### Options

```
  -h, --help     help for doctor
  -j, --json     output as JSON
      --repair   Auto-fix repairable issues
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

