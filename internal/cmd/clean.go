package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newCleanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Remove state directory (.siren/)",
		Long:  "Delete the .siren/ directory to reset to a clean state. Use 'sightjack init' to reinitialize.",
		Example: `  sightjack clean
  sightjack clean --yes`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			stateDir := filepath.Dir(cfgPath)

			info, err := os.Stat(stateDir)
			if err != nil || !info.IsDir() {
				fmt.Fprintf(cmd.ErrOrStderr(), "Nothing to clean at %s\n", stateDir)
				return nil
			}

			yes, _ := cmd.Flags().GetBool("yes")
			if !yes {
				fmt.Fprintf(cmd.ErrOrStderr(), "The following will be deleted:\n  %s/\n\nDelete? [y/N]: ", stateDir)
				var answer string
				fmt.Fscanln(cmd.InOrStdin(), &answer)
				if answer != "y" && answer != "Y" {
					fmt.Fprintf(cmd.ErrOrStderr(), "Aborted.\n")
					return nil
				}
			}

			if err := os.RemoveAll(stateDir); err != nil {
				return fmt.Errorf("remove %s: %w", stateDir, err)
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "Cleaned %s\n", stateDir)
			return nil
		},
	}
	cmd.Flags().Bool("yes", false, "Skip confirmation prompt")
	return cmd
}
