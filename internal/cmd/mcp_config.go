package cmd

import (
	"fmt"
	"strings"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/platform"
	"github.com/hironow/sightjack/internal/session"
	"github.com/spf13/cobra"
)

func newMCPConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp-config",
		Short: "Manage MCP wiring for Claude Code sessions",
		Long: `Manage the .mcp.json file that controls which MCP servers
are available to Claude Code sessions.

Use 'generate' to create the initial config, then edit it to add or remove
MCP servers as needed. Claude Code uses --strict-mcp-config to enforce this
allowlist when the file exists.`,
		Example: `  sightjack mcp-config generate
  sightjack mcp-config generate --force`,
	}

	cmd.AddCommand(newMCPConfigGenerateCommand())
	return cmd
}

func newMCPConfigGenerateCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "generate [path]",
		Short: "Generate .mcp.json and .claude/settings.json for Claude Code sessions",
		Long: `Generate .mcp.json and .claude/settings.json for Claude Code MCP sessions.

.mcp.json controls which MCP servers are available:
  - includes this repo's sightjack MCP server

.claude/settings.json disables plugins so the session uses only the configured MCP surface.

Claude Code uses --strict-mcp-config to enforce the MCP allowlist.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			baseDir := "."
			if len(args) > 0 {
				baseDir = args[0]
			}

			logger := platform.NewLogger(cmd.ErrOrStderr(), false)
			// Wave mode only post jun15 MCP pivot (Linear tracking removed).
			mode := domain.NewTrackingMode(false)

			path, genErr := session.GenerateMCPConfig(baseDir, mode, force)
			if genErr != nil {
				if strings.Contains(genErr.Error(), "already exists") {
					logger.Warn("mcp: %v", genErr)
					path = session.MCPConfigPath(baseDir)
				} else {
					return genErr
				}
			} else {
				logger.OK("Generated %s (mode: %s)", path, mode)
				logger.Info("Sightjack MCP server included. Edit to add custom servers.")
			}

			// Project-root .mcp.json upsert (refs issue 0032 C5): a bare
			// `claude` session in this project auto-attaches the server.
			// Merge-aware — sibling tools' entries survive. Lives outside
			// GenerateMCPConfig because mcp_config.go is canonical-locked.
			rootPath, rootErr := session.UpsertRootMCPConfig(baseDir)
			if rootErr != nil {
				return rootErr
			}
			logger.OK("Upserted %s (project-root, merge-aware)", rootPath)

			settingsPath, settingsErr := session.GenerateClaudeSettings(baseDir, force)
			if settingsErr != nil {
				if strings.Contains(settingsErr.Error(), "already exists") {
					logger.Warn("settings: %v", settingsErr)
				} else {
					return fmt.Errorf("settings: %w", settingsErr)
				}
			} else {
				logger.OK("Generated %s", settingsPath)
			}

			fmt.Fprintln(cmd.OutOrStdout(), path)
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing .mcp.json")
	return cmd
}
