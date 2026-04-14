package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/platform"
	"github.com/hironow/sightjack/internal/session"
	"github.com/hironow/sightjack/internal/usecase"
	"github.com/hironow/sightjack/internal/usecase/port"
)

func newRunCommand() *cobra.Command {
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
			baseDir, err := resolveTargetDir(args)
			if err != nil {
				return fmt.Errorf("invalid path: %w", err)
			}
			// Acquire daemon lock — prevents multiple instances on the same directory
			runDir := filepath.Join(baseDir, domain.StateDir, ".run")
			unlock, lockErr := session.TryLockDaemon(runDir)
			if lockErr != nil {
				return fmt.Errorf("daemon lock: %w", lockErr)
			}
			defer unlock()

			cfg, err := loadConfig(cmd, baseDir)
			if err != nil {
				return err
			}
			// Preflight: verify required binaries exist
			bins := []string{"git"}
			if !dryRun {
				bins = append(bins, cfg.ClaudeCmd)
			}
			if cfg.Mode.IsWave() && !dryRun {
				bins = append(bins, "gh")
			}
			if err := session.PreflightCheck(bins...); err != nil {
				return err
			}

			// Initialize process-wide circuit breaker for rate limit / server error protection
			session.SetCircuitBreaker(platform.NewCircuitBreaker(logger))

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
			if cmd.Flags().Changed("strictness") {
				s, _ := cmd.Flags().GetString("strictness")
				level := domain.StrictnessLevel(s)
				if !level.Valid() {
					return fmt.Errorf("invalid strictness %q: must be fog, alert, or lockdown", s)
				}
				cfg.Strictness.Default = level
				logger.Info("Strictness override: %s", s)
			}
			if cmd.Flags().Changed("idle-timeout") {
				cfg.Gate.IdleTimeout, _ = cmd.Flags().GetDuration("idle-timeout")
			}

			// Validate base directory via domain primitive
			if _, rpErr := domain.NewRepoPath(baseDir); rpErr != nil {
				return rpErr
			}

			runner := session.NewSessionRunnerAdapter()
			factory := session.NewRecorderFactoryAdapter()

			// Cutover wiring: ensure SeqNr allocation is active
			if !dryRun {
				seqCounter, cutoverErr := session.EnsureCutover(cmd.Context(), baseDir, "sightjack.state", logger)
				if cutoverErr != nil {
					return fmt.Errorf("cutover: %w", cutoverErr)
				}
				defer seqCounter.Close()
				factory.SetSeqCounter(seqCounter)
			}

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
						if resumableState != nil && resumableState.AllWavesCompleted() {
							choice = domain.ResumeChoiceRescan
							logger.Info("Auto-approve: previous session fully completed, rescanning")
						} else {
							choice = domain.ResumeChoiceResume
							logger.Info("Auto-approve: resuming previous session")
						}
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
						resumeStore := factory.NewSessionEventStore(factory.SessionEventsDir(baseDir, resumableSessionID), logger)
						emitter := usecase.BuildSessionEmitter(cmd.Context(), resumeStore, logger, false, cfg, &platform.OTelPolicyMetrics{}, runner, resumableState.SessionID)
						return runner.RunResumeSession(cmd.Context(), cfg, baseDir, resumableState, cmd.InOrStdin(), cmd.OutOrStdout(), emitter, logger)
					case domain.ResumeChoiceRescan:
						rescanID := fmt.Sprintf("session-%d-%d", time.Now().UnixMilli(), os.Getpid())
						rescanStore := factory.NewSessionEventStore(factory.SessionEventsDir(baseDir, rescanID), logger)
						emitter := usecase.BuildSessionEmitter(cmd.Context(), rescanStore, logger, false, cfg, &platform.OTelPolicyMetrics{}, runner, rescanID)
						return runner.RunRescanSession(cmd.Context(), cfg, baseDir, promptState, rescanID, cmd.InOrStdin(), cmd.OutOrStdout(), emitter, logger)
					case domain.ResumeChoiceNew:
						goto freshSession
					}
				}
			}
		freshSession:

			sessionID := fmt.Sprintf("session-%d-%d", time.Now().UnixMilli(), os.Getpid())
			var sessionInput io.Reader
			var store port.EventStore
			if !dryRun {
				sessionInput = cmd.InOrStdin()
				store = factory.NewSessionEventStore(factory.SessionEventsDir(baseDir, sessionID), logger)
			}
			emitter := usecase.BuildSessionEmitter(cmd.Context(), store, logger, dryRun, cfg, &platform.OTelPolicyMetrics{}, runner, sessionID)
			return runner.RunSession(cmd.Context(), cfg, baseDir, sessionID, dryRun, sessionInput, cmd.OutOrStdout(), emitter, logger)
		},
	}

	cmd.Flags().String("notify-cmd", "", "Notification command ({title}, {message} placeholders)")
	cmd.Flags().String("approve-cmd", "", "Approval command ({message} placeholder, exit 0 = approve)")
	cmd.Flags().Bool("auto-approve", false, "Skip all interactive prompts (resume session + convergence gate)")
	cmd.Flags().String("review-cmd", "", "Review command (exit 0 = pass, non-zero = comments found)")
	cmd.Flags().String("session-mode", "", "Session mode: resume, new, or rescan (skip interactive prompt)")
	cmd.Flags().StringP("strictness", "s", "", "Override default strictness level (fog, alert, lockdown)")
	cmd.Flags().Duration("idle-timeout", domain.DefaultIdleTimeout, "idle timeout — exit after no D-Mail activity (0 = 24h safety cap, negative = disable waiting)")

	return cmd
}
