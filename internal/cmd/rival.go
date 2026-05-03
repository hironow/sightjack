// Package cmd rival.go: cobra registration for the `sightjack rival ...`
// subcommand family (Phase 1.1B).
//
// Plan: refs/plans/2026-05-03-rival-contract-v1-1-extensions.md §"Phase 1.1B"
//
// `rival` is a parent command grouping Rival Contract v1.1 utilities. The
// only subcommand in v1.1 is `rival export reasons`, which projects a
// Rival Contract v1 specification onto OpenSPDD REASONS Canvas markdown.
package cmd

import (
	"github.com/spf13/cobra"
)

// newRivalCommand returns the root `rival` cobra command. The command has
// no Run handler; users must invoke a subcommand (`export`).
func newRivalCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rival",
		Short: "Rival Contract v1.1 utilities",
		Long: `Rival Contract v1.1 utilities.

Use 'rival export reasons' to project a Rival Contract v1 specification
into the OpenSPDD REASONS Canvas markdown shape.`,
		Example: `  # Project a stand-alone D-Mail file to REASONS Canvas markdown
  sightjack rival export reasons --input ./spec-auth_aaaaaaaa.md

  # Project the current revision for a wave to a file (JSON)
  sightjack rival export reasons --wave wave-x --format json --output canvas.json`,
	}
	cmd.AddCommand(newRivalExportCommand())
	return cmd
}

// newRivalExportCommand returns the `rival export` parent. Its only
// subcommand in v1.1 is `reasons`. Future v2 export targets (e.g. ADR
// export, OpenSPDD JSON pipeline) would attach here.
func newRivalExportCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export a Rival Contract to an interop format",
		Long: `Export a Rival Contract v1 specification to a downstream tool's format.

Available targets:
  reasons   OpenSPDD REASONS Canvas (markdown or JSON).`,
		Example: `  # OpenSPDD REASONS Canvas (markdown)
  sightjack rival export reasons --input ./spec.md

  # OpenSPDD REASONS Canvas (JSON)
  sightjack rival export reasons --input ./spec.md --format json`,
	}
	cmd.AddCommand(newRivalExportReasonsCommand())
	return cmd
}
