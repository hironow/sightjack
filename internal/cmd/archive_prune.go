package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hironow/sightjack/internal/domain"
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
  sightjack archive-prune --days 7 --execute

  # Rebuild archive index from existing files
  sightjack archive-prune --rebuild-index`,
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

			rebuildIndex, _ := cmd.Flags().GetBool("rebuild-index")
			if rebuildIndex {
				if execute {
					return fmt.Errorf("--rebuild-index cannot be combined with --execute")
				}
				stateDir := filepath.Join(baseDir, domain.StateDir)
				indexPath := filepath.Join(stateDir, "archive", "index.jsonl")
				iw := &session.IndexWriter{}
				n, rbErr := iw.Rebuild(indexPath, stateDir, "sightjack")
				if rbErr != nil {
					return fmt.Errorf("rebuild index: %w", rbErr)
				}
				logger.Info("Rebuilt index: %d entries → %s", n, indexPath)
				return nil
			}

			outputFmt, _ := cmd.Flags().GetString("output")

			files, err := session.ListExpiredArchive(baseDir, days, logger)
			if err != nil {
				return fmt.Errorf("failed to list archive: %w", err)
			}

			eventFiles, eventErr := session.ListExpiredEventFiles(cmd.Context(), baseDir, days)
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
					// Index archive candidates before deletion
					if len(files) > 0 {
						stateDir := filepath.Join(baseDir, domain.StateDir)
						indexSightjackArchive(files, stateDir, logger)
					}
					if len(files) > 0 {
						deleted, delErr := session.DeleteArchiveFiles(baseDir, files)
						if delErr != nil {
							return fmt.Errorf("archive prune failed: %w", delErr)
						}
						out.ArchiveDeleted = len(deleted)
					}
					if len(eventFiles) > 0 {
						deleted, delErr := session.PruneEventFiles(cmd.Context(), baseDir, eventFiles)
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

			yes, _ := cmd.Flags().GetBool("yes")
			totalFiles := len(files) + len(eventFiles)
			if !yes {
				fmt.Fprintf(errW, "\nDelete these %d file(s)? [y/N] ", totalFiles)
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

			// Index archive candidates before deletion
			if len(files) > 0 {
				stateDir := filepath.Join(baseDir, domain.StateDir)
				indexSightjackArchive(files, stateDir, logger)
			}

			if len(files) > 0 {
				deleted, delErr := session.DeleteArchiveFiles(baseDir, files)
				if delErr != nil {
					return fmt.Errorf("archive prune failed: %w", delErr)
				}
				fmt.Fprintf(errW, "Pruned %d d-mail file(s).\n", len(deleted))
			}

			if len(eventFiles) > 0 {
				deleted, delErr := session.PruneEventFiles(cmd.Context(), baseDir, eventFiles)
				if delErr != nil {
					return fmt.Errorf("event prune failed: %w", delErr)
				}
				fmt.Fprintf(errW, "Pruned %d event file(s).\n", len(deleted))
			}

			// Prune flushed outbox DB rows + incremental vacuum.
			pruned, pruneErr := session.PruneFlushedOutbox(cmd.Context(), baseDir)
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
	cmd.Flags().Bool("rebuild-index", false, "Rebuild the archive index from existing files")

	return cmd
}

func indexSightjackArchive(files []string, stateDir string, logger domain.Logger) {
	archiveDir := filepath.Join(stateDir, "archive")
	var indexEntries []domain.IndexEntry
	for _, f := range files {
		if filepath.Ext(f) != ".md" {
			continue
		}
		fullPath := filepath.Join(archiveDir, f)
		indexEntries = append(indexEntries, session.ExtractMeta(fullPath, stateDir, "sightjack"))
	}
	if len(indexEntries) == 0 {
		return
	}
	indexPath := filepath.Join(stateDir, "archive", "index.jsonl")
	iw := &session.IndexWriter{}
	if err := iw.Append(indexPath, indexEntries); err != nil {
		logger.Warn("index append: %v", err)
	} else {
		logger.Info("Indexed %d entries → %s", len(indexEntries), indexPath)
	}
}
