## sightjack mcp-config generate

Generate .mcp.json and .claude/settings.json for Claude Code sessions

### Synopsis

Generate .mcp.json and .claude/settings.json for Claude Code MCP sessions.

.mcp.json controls which MCP servers are available:
  - includes this repo's sightjack MCP server

.claude/settings.json disables plugins so the session uses only the configured MCP surface.

Claude Code uses --strict-mcp-config to enforce the MCP allowlist.

```
sightjack mcp-config generate [path] [flags]
```

### Options

```
      --force   Overwrite existing .mcp.json
  -h, --help    help for generate
```

### Options inherited from parent commands

```
  -c, --config string   Config file path (default ".siren/config.yaml")
  -n, --dry-run         Generate prompts without executing Claude
  -l, --lang string     Language override (ja/en)
      --no-color        Disable colored output (respects NO_COLOR env)
  -o, --output string   Output format: text, json (default "text")
  -q, --quiet           Suppress all stderr output
  -v, --verbose         Verbose logging
```

### SEE ALSO

* [sightjack mcp-config](sightjack_mcp-config.md)	 - Manage MCP wiring for Claude Code sessions

