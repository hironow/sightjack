package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/hironow/sightjack/internal/session"
)

// newMCPCommand exposes `sightjack mcp` as a stdio MCP server entry
// point for the refs/issues/0027 jun15 MCP pivot Phase 2a. A claude
// code interactive session loads this binary via --mcp-config and
// calls sightjack tools from inside the human-initiated subscription
// quota.
//
// Exposes ping + next_wave + get_scan_result
// (read the session scan dir) + update_strictness (atomic
// .siren/config.yaml write).
//
// Distinct from `sightjack mcp-config`, which writes the Claude Code
// MCP allowlist that points back to this stdio server.
func newMCPCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Run sightjack as an MCP server over stdio (scan/wave data plane + strictness control)",
		Long: `Start a Model Context Protocol server reading JSON-RPC 2.0
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
the Claude Code MCP allowlist that points back to this stdio server).`,
		Example: `  # Launch Claude Code with the sightjack MCP server attached
  claude --mcp-config '{"sightjack":{"command":"sightjack","args":["mcp"]}}'

  # Pipe a tools/list request manually (for debugging)
  echo '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | sightjack mcp`,
		RunE: func(cmd *cobra.Command, args []string) error {
			baseDir, err := os.Getwd()
			if err != nil {
				return err
			}
			srv := session.NewMCPServer(cmd.InOrStdin(), cmd.OutOrStdout(), nil).WithBaseDir(baseDir)
			return srv.Serve(cmd.Context())
		},
	}
}
