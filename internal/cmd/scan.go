package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/session"
	"github.com/hironow/sightjack/internal/usecase"
)

func newScanCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "scan [path]",
		Short: "Classify and deep-scan issues",
		Long: `Classify and deep-scan issues in the configured project.

In wave mode (default): queries GitHub Issues via gh CLI.
In linear mode (--linear): queries Linear issues via Claude MCP tools.

Produces a ScanResult with cluster classification, completeness scores,
and shibito warnings. Use --json to output structured JSON for piping
into downstream commands.`,
		Example: `  # Interactive scan with navigator display
  sightjack scan

  # Pipe workflow: scan → waves → show
  sightjack scan --json | sightjack waves | sightjack show

  # Scan a specific project directory
  sightjack scan /path/to/project`,
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
			if cmd.Flags().Changed("strictness") {
				s, _ := cmd.Flags().GetString("strictness")
				level := domain.StrictnessLevel(s)
				if !level.Valid() {
					return fmt.Errorf("invalid strictness %q: must be fog, alert, or lockdown", s)
				}
				cfg.Strictness.Default = level
				logger.Info("Strictness override: %s", s)
			}
			logger.Info("Starting sightjack scan...")
			logger.Info("Team: %s | Project: %s | Lang: %s", cfg.Tracker.Team, cfg.Tracker.Project, cfg.Lang)

			// When --json is set, stream Claude output to stderr so stdout stays clean for pipe.
			streamOut := cmd.OutOrStdout()
			if jsonOutput {
				streamOut = cmd.ErrOrStderr()
			}
			sessionID := fmt.Sprintf("scan-%d-%d", time.Now().UnixMilli(), os.Getpid())
			rp, rpErr := domain.NewRepoPath(baseDir)
			if rpErr != nil {
				return rpErr
			}
			scanCmd := domain.NewRunScanCommand(rp, dryRun)
			result, err := usecase.RunScan(cmd.Context(), scanCmd, cfg, baseDir, sessionID, dryRun, streamOut, logger, session.NewScanRunnerAdapter(), session.NewRecorderFactoryAdapter())
			if err != nil {
				_ = tryWriteHandover(cmd.Context(), err, baseDir, domain.HandoverState{
					Tool:       "sightjack",
					Operation:  "wave",
					InProgress: fmt.Sprintf("Scan in %s", baseDir),
					PartialState: map[string]string{
						"Team":       cfg.Tracker.Team,
						"Project":    cfg.Tracker.Project,
						"Strictness": string(cfg.Strictness.Default),
					},
				}, logger)
				return fmt.Errorf("scan failed: %w", err)
			}

			if dryRun {
				logger.OK("Dry-run complete. Check .siren/.run/ for generated prompts.")
				return nil
			}

			w := cmd.OutOrStdout()
			if jsonOutput {
				data, jsonErr := json.MarshalIndent(result, "", "  ")
				if jsonErr != nil {
					return fmt.Errorf("JSON marshal failed: %w", jsonErr)
				}
				fmt.Fprintln(w, string(data))
			} else {
				nav := session.RenderNavigator(result, cfg.Tracker.Project)
				fmt.Fprintln(w)
				fmt.Fprint(w, nav)
			}

			logger.OK("Scan complete. Overall completeness: %.0f%%", result.Completeness*100)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Output scan result as JSON")
	cmd.Flags().StringP("strictness", "s", "", "Override default strictness level (fog, alert, lockdown)")

	return cmd
}
