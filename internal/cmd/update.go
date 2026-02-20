package cmd

import (
	"fmt"

	"github.com/creativeprojects/go-selfupdate"
	"github.com/spf13/cobra"
)

func newUpdateCmd() *cobra.Command {
	var checkOnly bool

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Self-update sightjack to the latest release",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			updater, err := selfupdate.NewUpdater(selfupdate.Config{
				Validator: &selfupdate.ChecksumValidator{UniqueFilename: "checksums.txt"},
			})
			if err != nil {
				return fmt.Errorf("failed to create updater: %w", err)
			}

			latest, found, err := updater.DetectLatest(cmd.Context(), selfupdate.ParseSlug("hironow/sightjack"))
			if err != nil {
				return fmt.Errorf("failed to detect latest version: %w", err)
			}
			if !found {
				fmt.Fprintln(cmd.OutOrStdout(), "No release found.")
				return nil
			}

			if latest.LessOrEqual(version) {
				fmt.Fprintf(cmd.OutOrStdout(), "Already up to date (v%s).\n", version)
				return nil
			}

			if checkOnly {
				fmt.Fprintf(cmd.OutOrStdout(), "Update available: v%s → v%s\n", version, latest.Version())
				return nil
			}

			exe, err := selfupdate.ExecutablePath()
			if err != nil {
				return fmt.Errorf("failed to locate executable: %w", err)
			}

			if err := updater.UpdateTo(cmd.Context(), latest, exe); err != nil {
				return fmt.Errorf("update failed: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Updated to v%s\n", latest.Version())
			return nil
		},
	}

	cmd.Flags().BoolVar(&checkOnly, "check", false, "Check for updates without installing")

	return cmd
}
