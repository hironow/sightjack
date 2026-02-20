package cmd

import (
	"fmt"
	"os"

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
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			baseDir, err := resolveBaseDir(args)
			if err != nil {
				return fmt.Errorf("invalid path: %w", err)
			}
			ctx := startSpan(cmd)
			defer endSpan(ctx)

			files, err := sightjack.ListExpiredArchive(baseDir, days)
			if err != nil {
				return fmt.Errorf("failed to list archive: %w", err)
			}

			if len(files) == 0 {
				fmt.Fprintf(os.Stderr, "No expired files in archive (threshold: %d days).\n", days)
				return nil
			}

			for _, f := range files {
				fmt.Println(f)
			}
			fmt.Fprintf(os.Stderr, "\n%d file(s) older than %d days.\n", len(files), days)

			if !execute {
				fmt.Fprintln(os.Stderr, "(dry-run — pass --execute to delete)")
				return nil
			}

			deleted, err := sightjack.DeleteArchiveFiles(baseDir, files)
			if err != nil {
				return fmt.Errorf("prune failed: %w", err)
			}
			fmt.Fprintf(os.Stderr, "Pruned %d file(s).\n", len(deleted))
			return nil
		},
	}

	cmd.Flags().BoolVar(&execute, "execute", false, "Execute archive pruning (default: dry-run)")
	cmd.Flags().IntVar(&days, "days", 30, "Retention days for archive-prune")

	return cmd
}
