package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/platform"
	"github.com/hironow/sightjack/internal/session"
	"github.com/hironow/sightjack/internal/usecase"
	"github.com/hironow/sightjack/internal/usecase/port"
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
				bins = append(bins, cfg.Assistant.Command)
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

			// Parse base directory into domain primitive (used by all command constructions below)
			rp, rpErr := domain.NewRepoPath(baseDir)
			if rpErr != nil {
				return rpErr
			}

			runner := session.NewSessionRunnerAdapter()
			factory := session.NewRecorderFactoryAdapter()

			// Check for existing state (resume detection)
			// First try to find a resumable session; fall back to the latest
			// state for rescan/new choices.
			if !dryRun {
				// Find best resumable session (may differ from the latest)
				resumableState, resumableSessionID, _ := session.LoadLatestResumableState(cmd.Context(), baseDir, func(s *domain.SessionState) bool {
					return session.CanResume(baseDir, s)
				})
				// Find latest state for display and rescan (regardless of resumability)
				displayState, _, stateErr := session.LoadLatestState(cmd.Context(), baseDir)
				if stateErr == nil {
					// If a resumable session exists, prefer it for the prompt display
					promptState := displayState
					if resumableState != nil {
						promptState = resumableState
					}

					// Determine session choice: --session-mode flag, --auto-approve, or interactive prompt
					var choice domain.ResumeChoice
					sessionMode, _ := cmd.Flags().GetString("session-mode")
					if sessionMode != "" {
						parsed, parseErr := domain.ParseSessionMode(sessionMode)
						if parseErr != nil {
							return parseErr
						}
						choice = parsed
					} else if cfg.Gate.AutoApprove {
						choice = domain.ResumeChoiceResume
						logger.Info("Auto-approve: resuming previous session")
					} else {
						scanner := bufio.NewScanner(cmd.InOrStdin())
						for {
							prompted, promptErr := session.PromptResume(cmd.Context(), cmd.OutOrStdout(), scanner, baseDir, promptState)
							if promptErr == domain.ErrQuit {
								return nil
							}
							if promptErr != nil {
								logger.Warn("Invalid input: %v", promptErr)
								continue
							}
							choice = prompted
							break
						}
					}

					switch choice {
					case domain.ResumeChoiceResume:
						if resumableState == nil {
							logger.Warn("No resumable session found — starting fresh session instead.")
							goto freshSession
						}
						resumeRecorder, recErr := factory.NewSessionRecorder(factory.SessionEventsDir(baseDir, resumableSessionID), resumableSessionID, logger)
						if recErr != nil {
							return fmt.Errorf("resume recorder: %w", recErr)
						}
						resumeSID, _ := domain.NewSessionID(resumableSessionID)
						return usecase.ResumeSession(cmd.Context(), domain.NewResumeSessionCommand(rp, resumeSID), cfg, baseDir, resumableState, cmd.InOrStdin(), cmd.OutOrStdout(), resumeRecorder, logger, &platform.OTelPolicyMetrics{}, runner)
					case domain.ResumeChoiceRescan:
						rescanID := fmt.Sprintf("session-%d-%d", time.Now().UnixMilli(), os.Getpid())
						rescanRecorder, recErr := factory.NewSessionRecorder(factory.SessionEventsDir(baseDir, rescanID), rescanID, logger)
						if recErr != nil {
							return fmt.Errorf("rescan recorder: %w", recErr)
						}
						return usecase.RescanSession(cmd.Context(), domain.NewRunSessionCommand(rp, dryRun), cfg, baseDir, promptState, rescanID, cmd.InOrStdin(), cmd.OutOrStdout(), rescanRecorder, logger, &platform.OTelPolicyMetrics{}, runner)
					case domain.ResumeChoiceNew:
						goto freshSession
					}
				}
			}
		freshSession:

			sessionID := fmt.Sprintf("session-%d-%d", time.Now().UnixMilli(), os.Getpid())
			var sessionInput io.Reader
			var recorder port.Recorder = port.NopRecorder{}
			if !dryRun {
				sessionInput = cmd.InOrStdin()
				rec, recErr := factory.NewSessionRecorder(factory.SessionEventsDir(baseDir, sessionID), sessionID, logger)
				if recErr != nil {
					return fmt.Errorf("session recorder: %w", recErr)
				}
				recorder = rec
			}
			return usecase.RunSession(cmd.Context(), domain.NewRunSessionCommand(rp, dryRun), cfg, baseDir, sessionID, dryRun, sessionInput, cmd.OutOrStdout(), recorder, logger, &platform.OTelPolicyMetrics{}, runner)
		},
	}

	cmd.Flags().String("notify-cmd", "", "Notification command ({title}, {message} placeholders)")
	cmd.Flags().String("approve-cmd", "", "Approval command ({message} placeholder, exit 0 = approve)")
	cmd.Flags().Bool("auto-approve", false, "Skip all interactive prompts (resume session + convergence gate)")
	cmd.Flags().String("review-cmd", "", "Review command (exit 0 = pass, non-zero = comments found)")
	cmd.Flags().String("session-mode", "", "Session mode: resume, new, or rescan (skip interactive prompt)")

	return cmd
}
