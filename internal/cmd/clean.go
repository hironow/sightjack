package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/spf13/cobra"
)

func newCleanCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clean [path]",
		Short: "Remove state directory (.siren/)",
		Long:  "Delete the .siren/ directory to reset to a clean state. Use 'sightjack init' to reinitialize.",
		Example: `  # Clean current directory
  sightjack clean

  # Clean a specific project
  sightjack clean /path/to/project

  # Skip confirmation prompt
  sightjack clean --yes`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var stateDir string
			if len(args) > 0 {
				base, err := resolveBaseDir(args)
				if err != nil {
					return err
				}
				stateDir = filepath.Join(base, domain.StateDir)
			} else {
				stateDir = filepath.Dir(cfgPath)
			}

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
