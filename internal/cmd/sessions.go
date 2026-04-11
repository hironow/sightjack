package cmd

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"text/tabwriter"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/session"
	"github.com/hironow/sightjack/internal/usecase/port"
	"github.com/spf13/cobra"
)

func newSessionsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sessions",
		Short: "Manage AI coding sessions",
		Long:  "Manage AI coding session records. Sessions are tracked in SQLite\nand can be listed, filtered, and re-entered interactively.",
		Example: `  sightjack sessions list
  sightjack sessions list --status completed --limit 5
  sightjack sessions enter <session-record-id>
  sightjack sessions enter --provider-id <claude-session-id>`,
	}
	cmd.AddCommand(
		newSessionsListCommand(),
		newSessionsEnterCommand(),
	)
	return cmd
}

func newSessionsListCommand() *cobra.Command {
	var (
		statusFilter string
		limit        int
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List recorded coding sessions",
		RunE: func(cmd *cobra.Command, _ []string) error {
			logger := cmd.Context().Value(loggerKey).(domain.Logger)
			_, stateDirPath, err := resolveSessionsDir(cmd)
			if err != nil {
				return err
			}
			dbPath := filepath.Join(stateDirPath, ".run", "sessions.db")

			store, err := session.NewSQLiteCodingSessionStore(dbPath)
			if err != nil {
				return fmt.Errorf("open session store: %w", err)
			}
			defer store.Close()

			opts := port.ListSessionOpts{Limit: limit}
			if statusFilter != "" {
				s := domain.SessionStatus(statusFilter)
				opts.Status = &s
			}

			records, err := store.List(cmd.Context(), opts)
			if err != nil {
				return fmt.Errorf("list sessions: %w", err)
			}

			outputFmt, _ := cmd.Flags().GetString("output")
			if outputFmt == "json" {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(records)
			}

			if len(records) == 0 {
				logger.Info("No sessions found.")
				return nil
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tPROVIDER\tSTATUS\tMODEL\tPROVIDER_SESSION\tCREATED")
			for _, r := range records {
				pid := r.ProviderSessionID
				if len(pid) > 12 {
					pid = pid[:12] + "..."
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
					r.ID, r.Provider, r.Status, r.Model,
					pid, r.CreatedAt.Format("2006-01-02 15:04"))
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&statusFilter, "status", "", "Filter by status (running, completed, failed, abandoned)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Max results")
	cmd.Flags().String("path", "", "Repository root path")
	return cmd
}

func newSessionsEnterCommand() *cobra.Command {
	var providerID string
	cmd := &cobra.Command{
		Use:   "enter [session-record-id]",
		Short: "Re-enter an AI coding session interactively",
		Long:  "Launches the provider CLI in interactive mode with --resume, preserving isolation flags.\nPass a session record ID or use --provider-id for direct provider session targeting.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, stateDirPath, resolveErr := resolveSessionsDir(cmd)
			if resolveErr != nil {
				return resolveErr
			}
			dbPath := filepath.Join(stateDirPath, ".run", "sessions.db")

			store, err := session.NewSQLiteCodingSessionStore(dbPath)
			if err != nil {
				return fmt.Errorf("open session store: %w", err)
			}
			defer store.Close()

			var rec domain.CodingSessionRecord
			if len(args) == 1 {
				rec, err = store.Load(cmd.Context(), args[0])
				if err != nil {
					return fmt.Errorf("load session %q: %w", args[0], err)
				}
			} else if providerID != "" {
				rec, err = store.LatestByProviderSessionID(cmd.Context(), domain.ProviderClaudeCode, providerID)
				if err != nil {
					return fmt.Errorf("find session by provider ID %q: %w", providerID, err)
				}
			} else {
				return fmt.Errorf("provide a session record ID or --provider-id")
			}

			if rec.ProviderSessionID == "" {
				return fmt.Errorf("session %s has no provider session ID", rec.ID)
			}
			if rec.WorkDir == "" {
				return fmt.Errorf("session %s has no work directory recorded", rec.ID)
			}

			configPath := filepath.Join(stateDirPath, "config.yaml")
			if f := cmd.Flags().Lookup("config"); f != nil && cmd.Flags().Changed("config") {
				configPath, _ = cmd.Flags().GetString("config")
			}
			cfg, cfgErr := loadSessionsConfig(configPath)
			if cfgErr != nil {
				return fmt.Errorf("load config %s: %w", configPath, cfgErr)
			}

			enterCfg := session.EnterConfig{
				ProviderCmd:       cfg.ClaudeCmd,
				ProviderSessionID: rec.ProviderSessionID,
				WorkDir:           rec.WorkDir,
				ConfigBase:        repoRoot,
				Stdin:             cmd.InOrStdin(),
				Stdout:            cmd.OutOrStdout(),
				Stderr:            cmd.ErrOrStderr(),
			}
			return session.EnterSession(cmd.Context(), enterCfg)
		},
	}
	cmd.Flags().StringVar(&providerID, "provider-id", "", "Resume by provider session ID directly")
	cmd.Flags().String("path", "", "Repository root path")
	return cmd
}

