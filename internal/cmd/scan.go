package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	sightjack "github.com/hironow/sightjack"
	"github.com/hironow/sightjack/internal/eventsource"
	"github.com/hironow/sightjack/internal/session"
)

func newScanCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "scan [path]",
		Short: "Classify and deep-scan Linear issues",
		Long: `Classify and deep-scan Linear issues in the configured project.

Connects to the Linear API, fetches issues, and produces a ScanResult
with cluster classification, completeness scores, and shibito warnings.
Use --json to output structured JSON for piping into downstream commands.`,
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
			sessionID := fmt.Sprintf("scan-%d-%d", time.Now().UnixMilli(), os.Getpid())

			logger.Info("Starting sightjack scan...")
			logger.Info("Team: %s | Project: %s | Lang: %s", cfg.Linear.Team, cfg.Linear.Project, cfg.Lang)

			// When --json is set, stream Claude output to stderr so stdout stays clean for pipe.
			streamOut := cmd.OutOrStdout()
			if jsonOutput {
				streamOut = cmd.ErrOrStderr()
			}
			result, err := session.RunScan(cmd.Context(), cfg, baseDir, sessionID, dryRun, streamOut, logger)
			if err != nil {
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
				nav := session.RenderNavigator(result, cfg.Linear.Project)
				fmt.Fprintln(w)
				fmt.Fprint(w, nav)
			}

			// Cache scan result for pipe replay: cat .siren/.run/<id>/scan_result.json | sightjack waves
			scanResultPath := filepath.Join(sightjack.ScanDir(baseDir, sessionID), "scan_result.json")
			if err := session.WriteScanResult(scanResultPath, result); err != nil {
				logger.Warn("Failed to cache scan result: %v", err)
			}

			// Record events for state reconstruction
			var clusters []sightjack.ClusterState
			for _, c := range result.Clusters {
				clusters = append(clusters, sightjack.ClusterState{
					Name:         c.Name,
					Completeness: c.Completeness,
					IssueCount:   len(c.Issues),
				})
			}
			store := eventsource.NewFileEventStore(eventsource.EventStorePath(baseDir, sessionID))
			recorder, recErr := eventsource.NewSessionRecorder(store, sessionID)
			if recErr != nil {
				return fmt.Errorf("session recorder: %w", recErr)
			}
			if err := recorder.Record(sightjack.EventSessionStarted, sightjack.SessionStartedPayload{
				Project:         cfg.Linear.Project,
				StrictnessLevel: string(cfg.Strictness.Default),
			}); err != nil {
				logger.Warn("Failed to record session start: %v", err)
			}
			if err := recorder.Record(sightjack.EventScanCompleted, sightjack.ScanCompletedPayload{
				Clusters:       clusters,
				Completeness:   result.Completeness,
				ShibitoCount:   len(result.ShibitoWarnings),
				ScanResultPath: scanResultPath,
				LastScanned:    time.Now(),
			}); err != nil {
				logger.Warn("Failed to record scan completed: %v", err)
			} else {
				logger.OK("Events saved to %s", eventsource.EventStorePath(baseDir, sessionID))
			}

			logger.OK("Scan complete. Overall completeness: %.0f%%", result.Completeness*100)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Output scan result as JSON")

	return cmd
}
