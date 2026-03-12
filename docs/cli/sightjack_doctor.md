## sightjack doctor

Check environment and tool availability

### Synopsis

Check environment health and tool availability.

Verifies that the sightjack config is valid, required tools
(claude, git) are installed, and the Linear MCP connection
is working. Reports each check with one of four statuses:

- **OK** — check passed
- **FAIL** — check failed (exit code 1)
- **SKIP** — check skipped (dependency missing)
- **WARN** — advisory warning (exit code 0, not a failure)

The context-budget check estimates total token consumption and
provides per-item diagnostics when the threshold (20,000 tokens)
is exceeded. Categories include tools, skills, plugins, mcp, and
hooks. The heaviest category is marked, and a category-specific
hint is shown (e.g., recommending `.claude/settings.json` to
limit enabled plugins).

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
  -j, --json   output as JSON
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

