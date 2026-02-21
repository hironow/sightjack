package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	sightjack "github.com/hironow/sightjack"
)

func newSelectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "select",
		Short: "Interactively pick a wave from stdin WavePlan",
		Long: `Interactively pick a wave from a WavePlan JSON on stdin.

Presents available waves and prompts for selection via /dev/tty.
Outputs the selected wave as JSON with remaining sibling context
for downstream commands (apply, discuss).`,
		Example: `  # Select a wave and apply it
  sightjack scan --json | sightjack waves | sightjack select | sightjack apply

  # Select a wave and start a discussion
  sightjack scan --json | sightjack waves | sightjack select | sightjack discuss`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("failed to read stdin: %w", err)
			}
			if len(data) == 0 {
				return fmt.Errorf("no input on stdin. Pipe wave plan: sightjack waves | sightjack select")
			}

			var plan sightjack.WavePlan
			if err := json.Unmarshal(data, &plan); err != nil {
				return fmt.Errorf("invalid WavePlan JSON: %w", err)
			}

			if len(plan.Waves) == 0 {
				return fmt.Errorf("no waves in plan")
			}

			// Open terminal for interactive input (stdin is consumed by pipe).
			// Try /dev/tty first (Unix); fall back to cmd.InOrStdin() for
			// platforms without /dev/tty (e.g., Windows).
			var ttyCloser io.Closer
			tty, err := os.Open("/dev/tty")
			if err != nil {
				tty = nil
			} else {
				ttyCloser = tty
			}
			if ttyCloser != nil {
				defer ttyCloser.Close()
			}

			var input io.Reader
			if tty != nil {
				input = tty
			} else {
				input = cmd.InOrStdin()
			}
			scanner := bufio.NewScanner(input)
			available := sightjack.AvailableWaves(plan.Waves, map[string]bool{})

			if len(available) == 0 {
				return fmt.Errorf("no available waves (all locked or completed)")
			}

			selected, err := sightjack.PromptWaveSelection(cmd.Context(), os.Stderr, scanner, available)
			if err != nil {
				if err == sightjack.ErrQuit || err == sightjack.ErrGoBack {
					return nil
				}
				return fmt.Errorf("selection failed: %w", err)
			}

			// Attach cluster context from scan result if available.
			if plan.ScanResult != nil {
				for _, c := range plan.ScanResult.Clusters {
					if c.Name == selected.ClusterName {
						selected.ClusterContext = &c
						break
					}
				}
			}

			// Build remaining waves (all plan waves except the selected one)
			// so downstream apply → nextgen can accurately check NeedsMoreWaves.
			var remaining []sightjack.Wave
			selectedKey := sightjack.WaveKey(selected)
			for _, w := range plan.Waves {
				if sightjack.WaveKey(w) != selectedKey {
					remaining = append(remaining, w)
				}
			}

			// Output selected wave with remaining sibling context.
			type selectOutput struct {
				sightjack.Wave
				RemainingWaves []sightjack.Wave `json:"remaining_waves,omitempty"`
			}
			output := selectOutput{Wave: selected, RemainingWaves: remaining}
			out, jsonErr := json.MarshalIndent(output, "", "  ")
			if jsonErr != nil {
				return fmt.Errorf("JSON marshal failed: %w", jsonErr)
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(out))
			return nil
		},
	}
}
