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
	cmd := &cobra.Command{
		Use:   "run [path]",
		Short: "Interactive wave approval and apply loop",
		Long: `Run an interactive session with wave approval and apply loop.

Combines scan → waves → select → apply → nextgen in a single
interactive session. Supports resume from a previous session
if event data is found in .siren/events/.`,
		Example: `  # Start a new interactive session
  sightjack run

  # Resume a previous session (auto-detected)
  sightjack run

  # Dry-run mode (generate prompts without executing)
  sightjack run --dry-run

  # Auto-approve convergence gate
  sightjack run --auto-approve

  # Custom notification command
  sightjack run --notify-cmd 'echo {title}: {message}'`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := loggerFrom(cmd)
			baseDir, err := resolveBaseDir(args)
			if err != nil {
				return fmt.Errorf("invalid path: %w", err)
			}
			cfg, err := loadConfig(cmd, baseDir)
			if err != nil {
				return err
			}
			// Override gate config from flags (Changed = user explicitly set the flag)
			if cmd.Flags().Changed("notify-cmd") {
				cfg.Gate.NotifyCmd, _ = cmd.Flags().GetString("notify-cmd")
			}
			if cmd.Flags().Changed("approve-cmd") {
				cfg.Gate.ApproveCmd, _ = cmd.Flags().GetString("approve-cmd")
			}
			if cmd.Flags().Changed("auto-approve") {
				cfg.Gate.AutoApprove, _ = cmd.Flags().GetBool("auto-approve")
			}
			// Check for existing state (resume detection)
			if !dryRun {
				existingState, _, stateErr := sightjack.LoadLatestState(baseDir)
				if stateErr == nil {
					scanner := bufio.NewScanner(cmd.InOrStdin())
					for {
						choice, promptErr := sightjack.PromptResume(cmd.Context(), cmd.OutOrStdout(), scanner, existingState)
						if promptErr == sightjack.ErrQuit {
							return nil
						}
						if promptErr != nil {
							logger.Warn("Invalid input: %v", promptErr)
							continue
						}
						switch choice {
						case sightjack.ResumeChoiceResume:
							if !sightjack.CanResume(existingState) {
								logger.Warn("Cached scan data missing — starting fresh session instead.")
								goto freshSession
							}
							return sightjack.RunResumeSession(cmd.Context(), cfg, baseDir, existingState, cmd.InOrStdin(), cmd.OutOrStdout(), logger)
						case sightjack.ResumeChoiceRescan:
							return sightjack.RunRescanSession(cmd.Context(), cfg, baseDir, existingState, cmd.InOrStdin(), cmd.OutOrStdout(), logger)
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
			return sightjack.RunSession(cmd.Context(), cfg, baseDir, sessionID, dryRun, sessionInput, cmd.OutOrStdout(), logger)
		},
	}

	cmd.Flags().String("notify-cmd", "", "Notification command ({title}, {message} placeholders)")
	cmd.Flags().String("approve-cmd", "", "Approval command ({message} placeholder, exit 0 = approve)")
	cmd.Flags().Bool("auto-approve", false, "Skip approval gate for convergence D-Mail")

	return cmd
}
