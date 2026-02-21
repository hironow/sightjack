package cmd

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/creativeprojects/go-selfupdate"
	"github.com/spf13/cobra"
)

func newUpdateCmd() *cobra.Command {
	var checkOnly bool

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Self-update sightjack to the latest release",
		Long: `Self-update sightjack to the latest GitHub release.

Downloads the latest release, verifies the checksum, and replaces
the current binary. Use --check to only check for updates without
installing.`,
		Example: `  # Check for updates
  sightjack update --check

  # Update to the latest version
  sightjack update`,
		Args: cobra.NoArgs,
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

			// Guard: version may be "dev" for local builds (non-semver).
			// LessOrEqual calls semver.MustParse internally, which panics on invalid input.
			if _, err := semver.NewVersion(version); err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "Development build (version %q) — cannot compare versions.\nLatest release: v%s\n", version, latest.Version())
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

	cmd.Flags().BoolVarP(&checkOnly, "check", "C", false, "Check for updates without installing")

	return cmd
}
