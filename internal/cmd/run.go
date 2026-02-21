package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	sightjack "github.com/hironow/sightjack"
)

func newRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run [path]",
		Short: "Interactive wave approval and apply loop",
		Long: `Run an interactive session with wave approval and apply loop.

Combines scan → waves → select → apply → nextgen in a single
interactive session. Supports resume from a previous session
if state is found in .siren/state.json.`,
		Example: `  # Start a new interactive session
  sightjack run

  # Resume a previous session (auto-detected)
  sightjack run

  # Dry-run mode (generate prompts without executing)
  sightjack run --dry-run`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			baseDir, err := resolveBaseDir(args)
			if err != nil {
				return fmt.Errorf("invalid path: %w", err)
			}
			cfg, err := loadConfig(cmd, baseDir)
			if err != nil {
				return err
			}
			// Check for existing state (resume detection)
			if !dryRun {
				existingState, stateErr := sightjack.ReadState(baseDir)
				if stateErr != nil {
					recovered, recErr := sightjack.RecoverLatestState(baseDir)
					if recErr == nil {
						existingState = recovered
						stateErr = nil
					}
				}
				if stateErr == nil {
					scanner := bufio.NewScanner(cmd.InOrStdin())
					for {
						choice, promptErr := sightjack.PromptResume(cmd.Context(), cmd.OutOrStdout(), scanner, existingState)
						if promptErr == sightjack.ErrQuit {
							return nil
						}
						if promptErr != nil {
							sightjack.LogWarn("Invalid input: %v", promptErr)
							continue
						}
						switch choice {
						case sightjack.ResumeChoiceResume:
							if !sightjack.CanResume(existingState) {
								sightjack.LogWarn("Cached scan data missing — starting fresh session instead.")
								goto freshSession
							}
							return sightjack.RunResumeSession(cmd.Context(), cfg, baseDir, existingState, cmd.InOrStdin())
						case sightjack.ResumeChoiceRescan:
							return sightjack.RunRescanSession(cmd.Context(), cfg, baseDir, existingState, cmd.InOrStdin())
						case sightjack.ResumeChoiceNew:
							goto freshSession
						}
					}
				}
			}
		freshSession:

			sessionID := fmt.Sprintf("session-%d-%d", time.Now().UnixMilli(), os.Getpid())
			var sessionInput io.Reader
			if !dryRun {
				sessionInput = cmd.InOrStdin()
			}
			return sightjack.RunSession(cmd.Context(), cfg, baseDir, sessionID, dryRun, sessionInput)
		},
	}
}
