## sightjack mcp-config generate

Generate mcp-config.json for --strict-mcp-config isolation

### Synopsis

Generate a mcp-config.json file that controls which MCP servers
are available to Claude subprocess invocations.

In wave mode (default): generates empty config (no MCP servers).
In linear mode (--linear): includes Linear MCP server.

The generated file can be freely edited to add custom MCP servers.
Claude subprocess uses --strict-mcp-config to enforce this allowlist.

```
sightjack mcp-config generate [path] [flags]
```

### Options

```
      --force   Overwrite existing mcp-config.json
  -h, --help    help for generate
```

### Options inherited from parent commands

```
  -c, --config string   Config file path (default ".siren/config.yaml")
  -n, --dry-run         Generate prompts without executing Claude
  -l, --lang string     Language override (ja/en)
      --linear          Use Linear MCP for issue tracking (default: wave-centric mode)
      --no-color        Disable colored output (respects NO_COLOR env)
  -o, --output string   Output format: text, json (default "text")
  -q, --quiet           Suppress all stderr output
  -v, --verbose         Verbose logging
```

### SEE ALSO

* [sightjack mcp-config](sightjack_mcp-config.md)  - Manage MCP configuration for Claude subprocess isolation
