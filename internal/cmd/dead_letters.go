package cmd

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/session"
)

func newDeadLettersCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dead-letters",
		Short: "Manage dead-lettered outbox items",
		Long: `Manage outbox items that have exceeded the maximum retry count
and are permanently stuck.

Use the purge subcommand to remove dead-lettered items.`,
		Example: `  # Show dead-letter count (dry-run)
  sightjack dead-letters purge

  # Remove dead-lettered items
  sightjack dead-letters purge --execute --yes`,
	}

	cmd.AddCommand(newDeadLettersPurgeCommand())

	return cmd
}

func newDeadLettersPurgeCommand() *cobra.Command {
	var (
		execute bool
	)

	cmd := &cobra.Command{
		Use:   "purge [path]",
		Short: "Remove dead-lettered outbox items",
		Long: `Remove outbox items that have failed delivery 3+ times.

By default, runs in dry-run mode showing the count of dead-lettered items.
Pass --execute to actually remove them.`,
		Example: `  # Dry-run: show dead letter count
  sightjack dead-letters purge

  # Remove dead-lettered items
  sightjack dead-letters purge --execute

  # Skip confirmation prompt
  sightjack dead-letters purge --execute --yes

  # JSON output for scripting
  sightjack dead-letters purge -o json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if execute && dryRun {
				return fmt.Errorf("--execute and --dry-run are mutually exclusive")
			}
			baseDir, err := resolveTargetDir(args)
			if err != nil {
				return fmt.Errorf("invalid path: %w", err)
			}
			logger := loggerFrom(cmd)
			outputFmt, _ := cmd.Flags().GetString("output")

			// Guard: do not create DB as side-effect if it does not exist yet.
			dbPath := filepath.Join(baseDir, domain.StateDir, ".run", "outbox.db")
			if _, statErr := os.Stat(dbPath); errors.Is(statErr, fs.ErrNotExist) {
				if outputFmt == "json" {
					fmt.Fprintln(cmd.OutOrStdout(), `{"dead_letters":0,"purged":0}`)
					return nil
				}
				fmt.Fprintln(cmd.ErrOrStderr(), "No dead-lettered outbox items.")
				return nil
			}

			store, err := session.NewOutboxStoreForDir(baseDir)
			if err != nil {
				return fmt.Errorf("open outbox store: %w", err)
			}
			defer store.Close()

			ctx := cmd.Context()
			count, err := store.DeadLetterCount(ctx)
			if err != nil {
				return fmt.Errorf("dead letter count: %w", err)
			}

			if outputFmt == "json" {
				out := struct {
					DeadLetters int `json:"dead_letters"`
					Purged      int `json:"purged"`
				}{
					DeadLetters: count,
				}
				if execute && count > 0 {
					purged, purgeErr := store.PurgeDeadLetters(ctx)
					if purgeErr != nil {
						return fmt.Errorf("purge dead letters: %w", purgeErr)
					}
					out.Purged = purged
					if vacErr := store.IncrementalVacuum(); vacErr != nil {
						logger.Warn("incremental vacuum: %v", vacErr)
					}
				}
				data, jsonErr := json.Marshal(out)
				if jsonErr != nil {
					return jsonErr
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}

			// text output — metadata to stderr
			errW := cmd.ErrOrStderr()

			if count == 0 {
				fmt.Fprintln(errW, "No dead-lettered outbox items.")
				return nil
			}

			fmt.Fprintf(errW, "%d dead-lettered outbox item(s).\n", count)

			if !execute {
				fmt.Fprintln(errW, "(dry-run — pass --execute to purge)")
				return nil
			}

			yes, _ := cmd.Flags().GetBool("yes")
			if !yes {
				fmt.Fprintf(errW, "\nPurge %d dead-lettered item(s)? [y/N] ", count)
				scanner := bufio.NewScanner(cmd.InOrStdin())
				if !scanner.Scan() {
					if scanErr := scanner.Err(); scanErr != nil {
						return fmt.Errorf("read confirmation: %w", scanErr)
					}
					fmt.Fprintln(errW, "Cancelled.")
					return nil
				}
				answer := strings.TrimSpace(scanner.Text())
				if answer != "y" && answer != "Y" {
					fmt.Fprintln(errW, "Cancelled.")
					return nil
				}
			}

			purged, purgeErr := store.PurgeDeadLetters(ctx)
			if purgeErr != nil {
				return fmt.Errorf("purge dead letters: %w", purgeErr)
			}
			fmt.Fprintf(errW, "Purged %d dead-lettered item(s).\n", purged)

			if vacErr := store.IncrementalVacuum(); vacErr != nil {
				logger.Warn("incremental vacuum: %v", vacErr)
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&execute, "execute", "x", false, "Execute purge (default: dry-run)")
	cmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")

	return cmd
}
