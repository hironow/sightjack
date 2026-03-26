## sightjack mcp-config

Manage MCP configuration for Claude subprocess isolation

### Synopsis

Manage the mcp-config.json file that controls which MCP servers
are available to Claude subprocess invocations.

Use 'generate' to create the initial config, then edit it to add or remove
MCP servers as needed. Claude subprocess uses --strict-mcp-config to enforce
this allowlist when the file exists.

### Examples

```
  sightjack mcp-config generate
  sightjack mcp-config generate --linear
  sightjack mcp-config generate --force
```

### Options

```
  -h, --help   help for mcp-config
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

* [sightjack](sightjack.md)  - SIREN-inspired issue architecture tool for Linear
* [sightjack mcp-config generate](sightjack_mcp-config_generate.md)  - Generate mcp-config.json for --strict-mcp-config isolation
