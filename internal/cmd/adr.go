package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	sightjack "github.com/hironow/sightjack"
)

func newADRCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "adr [path]",
		Short: "Generate ADR Markdown from stdin DiscussResult",
		Long: `Generate an Architecture Decision Record from a DiscussResult on stdin.

Reads a DiscussResult JSON (from 'discuss') and renders it as
Markdown in the standard ADR format with auto-numbered filename.
Output is written to stdout for redirection to docs/adr/.`,
		Example: `  # Full pipeline: discuss → adr → file
  sightjack discuss | sightjack adr > docs/adr/0042-my-decision.md

  # Generate ADR from a saved discussion
  cat discussion.json | sightjack adr`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			baseDir, err := resolveBaseDir(args)
			if err != nil {
				return fmt.Errorf("invalid path: %w", err)
			}
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("failed to read stdin: %w", err)
			}
			if len(data) == 0 {
				return fmt.Errorf("no input on stdin. Pipe discuss result: sightjack discuss | sightjack adr")
			}

			var dr sightjack.DiscussResult
			if err := json.Unmarshal(data, &dr); err != nil {
				return fmt.Errorf("invalid DiscussResult JSON: %w", err)
			}

			adrDir := sightjack.ADRDir(baseDir)
			adrNum, err := sightjack.NextADRNumber(adrDir)
			if err != nil {
				return fmt.Errorf("failed to determine ADR number: %w", err)
			}

			md := sightjack.RenderADRFromDiscuss(dr, adrNum)
			fmt.Print(md)
			return nil
		},
	}
}
