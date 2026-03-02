package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hironow/sightjack/internal/session"
)

func newArchivePruneCmd() *cobra.Command {
	var (
		execute bool
		days    int
	)

	cmd := &cobra.Command{
		Use:   "archive-prune [path]",
		Short: "Prune expired d-mails and event files",
		Long: `Prune expired d-mails from the archive directory and
expired event files from the events directory.

Lists archived d-mail files and event files older than the retention threshold.
By default, runs in dry-run mode showing what would be deleted.
Pass --execute to actually remove the files.`,
		Example: `  # Dry-run: list expired files (default 30 days)
  sightjack archive-prune

  # Delete expired files
  sightjack archive-prune --execute

  # JSON output for scripting
  sightjack archive-prune -o json

  # Custom retention period
  sightjack archive-prune --days 7 --execute`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if execute && dryRun {
				return fmt.Errorf("--execute and --dry-run are mutually exclusive")
			}
			baseDir, err := resolveBaseDir(args)
			if err != nil {
				return fmt.Errorf("invalid path: %w", err)
			}
			logger := loggerFrom(cmd)
			outputFmt, _ := cmd.Flags().GetString("output")

			files, err := session.ListExpiredArchive(baseDir, days, logger)
			if err != nil {
				return fmt.Errorf("failed to list archive: %w", err)
			}

			eventFiles, eventErr := session.ListExpiredEventFiles(baseDir, days)
			if eventErr != nil {
				logger.Warn("Failed to list expired events: %v", eventErr)
			}

			if outputFmt == "json" {
				out := struct {
					ArchiveCandidates int      `json:"archive_candidates"`
					ArchiveDeleted    int      `json:"archive_deleted"`
					ArchiveFiles      []string `json:"archive_files"`
					EventCandidates   int      `json:"event_candidates"`
					EventDeleted      int      `json:"event_deleted"`
					EventFiles        []string `json:"event_files"`
				}{
					ArchiveCandidates: len(files),
					ArchiveFiles:      files,
					EventCandidates:   len(eventFiles),
					EventFiles:        eventFiles,
				}
				if execute {
					if len(files) > 0 {
						deleted, delErr := session.DeleteArchiveFiles(baseDir, files)
						if delErr != nil {
							return fmt.Errorf("archive prune failed: %w", delErr)
						}
						out.ArchiveDeleted = len(deleted)
					}
					if len(eventFiles) > 0 {
						deleted, delErr := session.PruneEventFiles(baseDir, eventFiles)
						if delErr != nil {
							return fmt.Errorf("event prune failed: %w", delErr)
						}
						out.EventDeleted = len(deleted)
					}
				}
				data, jsonErr := json.Marshal(out)
				if jsonErr != nil {
					return jsonErr
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}

			// text output — all metadata to stderr
			errW := cmd.ErrOrStderr()

			if len(files) == 0 {
				fmt.Fprintf(errW, "No expired d-mail files (threshold: %d days).\n", days)
			} else {
				fmt.Fprintln(errW, "Expired d-mail files:")
				for _, f := range files {
					fmt.Fprintln(errW, "  "+f)
				}
				fmt.Fprintf(errW, "%d d-mail file(s) older than %d days.\n", len(files), days)
			}

			if len(eventFiles) == 0 {
				fmt.Fprintf(errW, "No expired event files (threshold: %d days).\n", days)
			} else {
				fmt.Fprintln(errW, "Expired event files:")
				for _, f := range eventFiles {
					fmt.Fprintln(errW, "  "+f)
				}
				fmt.Fprintf(errW, "%d event file(s) older than %d days.\n", len(eventFiles), days)
			}

			if len(files) == 0 && len(eventFiles) == 0 {
				return nil
			}

			if !execute {
				fmt.Fprintln(errW, "(dry-run — pass --execute to delete)")
				return nil
			}

			if len(files) > 0 {
				deleted, delErr := session.DeleteArchiveFiles(baseDir, files)
				if delErr != nil {
					return fmt.Errorf("archive prune failed: %w", delErr)
				}
				fmt.Fprintf(errW, "Pruned %d d-mail file(s).\n", len(deleted))
			}

			if len(eventFiles) > 0 {
				deleted, delErr := session.PruneEventFiles(baseDir, eventFiles)
				if delErr != nil {
					return fmt.Errorf("event prune failed: %w", delErr)
				}
				fmt.Fprintf(errW, "Pruned %d event file(s).\n", len(deleted))
			}

			// Prune flushed outbox DB rows + incremental vacuum.
			pruned, pruneErr := session.PruneFlushedOutbox(baseDir)
			if pruneErr != nil {
				logger.Warn("Failed to prune outbox DB: %v", pruneErr)
			} else if pruned > 0 {
				fmt.Fprintf(errW, "Pruned %d flushed outbox row(s).\n", pruned)
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&execute, "execute", "x", false, "Execute archive pruning (default: dry-run)")
	cmd.Flags().IntVarP(&days, "days", "d", 30, "Retention days for archive-prune")
	cmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")

	return cmd
}
