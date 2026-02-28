package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	sightjack "github.com/hironow/sightjack"
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
			// Preflight: verify required binaries exist
			bins := []string{"git"}
			if !dryRun {
				bins = append(bins, cfg.Claude.Command)
			}
			if err := session.PreflightCheck(bins...); err != nil {
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
			if cmd.Flags().Changed("review-cmd") {
				cfg.Gate.ReviewCmd, _ = cmd.Flags().GetString("review-cmd")
			}
			// Check for existing state (resume detection)
			// First try to find a resumable session; fall back to the latest
			// state for rescan/new choices.
			if !dryRun {
				// Find best resumable session (may differ from the latest)
				resumableState, resumableSessionID, _ := session.LoadLatestResumableState(baseDir, func(s *sightjack.SessionState) bool {
					return session.CanResume(baseDir, s)
				})
				// Find latest state for display and rescan (regardless of resumability)
				displayState, _, stateErr := session.LoadLatestState(baseDir)
				if stateErr == nil {
					// If a resumable session exists, prefer it for the prompt display
					promptState := displayState
					if resumableState != nil {
						promptState = resumableState
					}
					scanner := bufio.NewScanner(cmd.InOrStdin())
					for {
						choice, promptErr := session.PromptResume(cmd.Context(), cmd.OutOrStdout(), scanner, baseDir, promptState)
						if promptErr == session.ErrQuit {
							return nil
						}
						if promptErr != nil {
							logger.Warn("Invalid input: %v", promptErr)
							continue
						}
						switch choice {
						case sightjack.ResumeChoiceResume:
							if resumableState == nil {
								logger.Warn("No resumable session found — starting fresh session instead.")
								goto freshSession
							}
							resumeStore := session.NewEventStore(baseDir, resumableSessionID)
							resumeRecorder, recErr := session.NewSessionRecorder(resumeStore, resumableSessionID)
							if recErr != nil {
								return fmt.Errorf("resume recorder: %w", recErr)
							}
							return session.RunResumeSession(cmd.Context(), cfg, baseDir, resumableState, cmd.InOrStdin(), cmd.OutOrStdout(), resumeRecorder, logger)
						case sightjack.ResumeChoiceRescan:
							rescanID := fmt.Sprintf("session-%d-%d", time.Now().UnixMilli(), os.Getpid())
							rescanStore := session.NewEventStore(baseDir, rescanID)
							rescanRecorder, recErr := session.NewSessionRecorder(rescanStore, rescanID)
							if recErr != nil {
								return fmt.Errorf("rescan recorder: %w", recErr)
							}
							return session.RunRescanSession(cmd.Context(), cfg, baseDir, promptState, rescanID, cmd.InOrStdin(), cmd.OutOrStdout(), rescanRecorder, logger)
						case sightjack.ResumeChoiceNew:
							goto freshSession
						}
					}
				}
			}
		freshSession:

			sessionID := fmt.Sprintf("session-%d-%d", time.Now().UnixMilli(), os.Getpid())
			var sessionInput io.Reader
			var recorder sightjack.Recorder = session.NopRecorder{}
			if !dryRun {
				sessionInput = cmd.InOrStdin()
				sessionStore := session.NewEventStore(baseDir, sessionID)
				rec, recErr := session.NewSessionRecorder(sessionStore, sessionID)
				if recErr != nil {
					return fmt.Errorf("session recorder: %w", recErr)
				}
				recorder = rec
			}
			return session.RunSession(cmd.Context(), cfg, baseDir, sessionID, dryRun, sessionInput, cmd.OutOrStdout(), recorder, logger)
		},
	}

	cmd.Flags().String("notify-cmd", "", "Notification command ({title}, {message} placeholders)")
	cmd.Flags().String("approve-cmd", "", "Approval command ({message} placeholder, exit 0 = approve)")
	cmd.Flags().Bool("auto-approve", false, "Skip approval gate for convergence D-Mail")
	cmd.Flags().String("review-cmd", "", "Review command (exit 0 = pass, non-zero = comments found)")

	return cmd
}
