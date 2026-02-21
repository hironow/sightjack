package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	sightjack "github.com/hironow/sightjack"
)

func newArchivePruneCmd() *cobra.Command {
	var (
		execute bool
		days    int
	)

	cmd := &cobra.Command{
		Use:   "archive-prune [path]",
		Short: "Prune expired d-mails from archive",
		Long: `Prune expired d-mails from the archive directory.

Lists archived d-mail files older than the retention threshold.
By default, runs in dry-run mode showing what would be deleted.
Pass --execute to actually remove the files.`,
		Example: `  # Dry-run: list expired files (default 30 days)
  sightjack archive-prune

  # Delete expired files
  sightjack archive-prune --execute

  # Custom retention period
  sightjack archive-prune --days 7 --execute`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			baseDir, err := resolveBaseDir(args)
			if err != nil {
				return fmt.Errorf("invalid path: %w", err)
			}
			files, err := sightjack.ListExpiredArchive(baseDir, days)
			if err != nil {
				return fmt.Errorf("failed to list archive: %w", err)
			}

			errW := cmd.ErrOrStderr()
			if len(files) == 0 {
				fmt.Fprintf(errW, "No expired files in archive (threshold: %d days).\n", days)
				return nil
			}

			w := cmd.OutOrStdout()
			for _, f := range files {
				fmt.Fprintln(w, f)
			}
			fmt.Fprintf(errW, "\n%d file(s) older than %d days.\n", len(files), days)

			if !execute {
				fmt.Fprintln(errW, "(dry-run — pass --execute to delete)")
				return nil
			}

			deleted, err := sightjack.DeleteArchiveFiles(baseDir, files)
			if err != nil {
				return fmt.Errorf("prune failed: %w", err)
			}
			fmt.Fprintf(errW, "Pruned %d file(s).\n", len(deleted))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&execute, "execute", "x", false, "Execute archive pruning (default: dry-run)")
	cmd.Flags().IntVarP(&days, "days", "d", 30, "Retention days for archive-prune")

	return cmd
}
