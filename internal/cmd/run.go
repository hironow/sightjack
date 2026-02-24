package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	sightjack "github.com/hironow/sightjack"
	"github.com/hironow/sightjack/internal/eventsource"
	"github.com/hironow/sightjack/internal/session"
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
				existingState, _, stateErr := eventsource.LoadLatestState(baseDir)
				if stateErr == nil {
					scanner := bufio.NewScanner(cmd.InOrStdin())
					for {
						choice, promptErr := session.PromptResume(cmd.Context(), cmd.OutOrStdout(), scanner, existingState)
						if promptErr == session.ErrQuit {
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
							resumeStore := eventsource.NewFileEventStore(eventsource.EventStorePath(baseDir, existingState.SessionID))
							resumeRecorder := eventsource.NewSessionRecorder(resumeStore, existingState.SessionID)
							return session.RunResumeSession(cmd.Context(), cfg, baseDir, existingState, cmd.InOrStdin(), cmd.OutOrStdout(), resumeRecorder, logger)
						case sightjack.ResumeChoiceRescan:
							rescanID := fmt.Sprintf("session-%d-%d", time.Now().UnixMilli(), os.Getpid())
							rescanStore := eventsource.NewFileEventStore(eventsource.EventStorePath(baseDir, rescanID))
							rescanRecorder := eventsource.NewSessionRecorder(rescanStore, rescanID)
							return session.RunRescanSession(cmd.Context(), cfg, baseDir, existingState, rescanID, cmd.InOrStdin(), cmd.OutOrStdout(), rescanRecorder, logger)
						case sightjack.ResumeChoiceNew:
							goto freshSession
						}
					}
				}
			}
		freshSession:

			sessionID := fmt.Sprintf("session-%d-%d", time.Now().UnixMilli(), os.Getpid())
			var sessionInput io.Reader
			var recorder sightjack.Recorder = sightjack.NopRecorder{}
			if !dryRun {
				sessionInput = cmd.InOrStdin()
				sessionStore := eventsource.NewFileEventStore(eventsource.EventStorePath(baseDir, sessionID))
				recorder = eventsource.NewSessionRecorder(sessionStore, sessionID)
			}
			return session.RunSession(cmd.Context(), cfg, baseDir, sessionID, dryRun, sessionInput, cmd.OutOrStdout(), recorder, logger)
		},
	}

	cmd.Flags().String("notify-cmd", "", "Notification command ({title}, {message} placeholders)")
	cmd.Flags().String("approve-cmd", "", "Approval command ({message} placeholder, exit 0 = approve)")
	cmd.Flags().Bool("auto-approve", false, "Skip approval gate for convergence D-Mail")

	return cmd
}
