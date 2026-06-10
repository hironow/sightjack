## sightjack mcp

Run sightjack as an MCP server over stdio (scan/wave data plane + strictness control)

### Synopsis

Start a Model Context Protocol server reading JSON-RPC 2.0
messages on stdin and writing responses on stdout.

Designed for embedding in a Claude Code interactive session via
--mcp-config so inference stays on the session's subscription quota
rather than crossing into the Agent SDK credit pool that gates
'claude -p' from 2026-06-15.

Exposes ping, next_wave + get_scan_result
(read the session's scan dir under .siren/.run/<session_id>/), and
update_strictness (atomically updates the strictness default
in .siren/config.yaml).

Not to be confused with 'sightjack mcp-config' (subcommand writing
the Claude Code MCP allowlist that points back to this stdio server).

```
sightjack mcp [flags]
```

### Examples

```
  # Launch Claude Code with the sightjack MCP server attached
  claude --mcp-config '{"sightjack":{"command":"sightjack","args":["mcp"]}}'

  # Pipe a tools/list request manually (for debugging)
  echo '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | sightjack mcp
```

### Options

```
  -h, --help   help for mcp
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

* [sightjack](sightjack.md)	 - SIREN-inspired issue architecture MCP data plane

