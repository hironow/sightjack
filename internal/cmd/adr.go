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
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			baseDir, err := resolveBaseDir(args)
			if err != nil {
				return fmt.Errorf("invalid path: %w", err)
			}
			ctx := startSpan(cmd)
			defer endSpan(ctx)

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
