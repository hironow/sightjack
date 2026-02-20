package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	sightjack "github.com/hironow/sightjack"
)

func newScanCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "scan [path]",
		Short: "Classify and deep-scan Linear issues",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			baseDir, err := resolveBaseDir(args)
			if err != nil {
				return fmt.Errorf("invalid path: %w", err)
			}
			cfg, err := loadConfig(cmd, baseDir)
			if err != nil {
				return err
			}
			sessionID := fmt.Sprintf("scan-%d-%d", time.Now().UnixMilli(), os.Getpid())

			sightjack.LogInfo("Starting sightjack scan...")
			sightjack.LogInfo("Team: %s | Project: %s | Lang: %s", cfg.Linear.Team, cfg.Linear.Project, cfg.Lang)

			result, err := sightjack.RunScan(cmd.Context(), cfg, baseDir, sessionID, dryRun)
			if err != nil {
				return fmt.Errorf("scan failed: %w", err)
			}

			if dryRun {
				sightjack.LogOK("Dry-run complete. Check .siren/.run/ for generated prompts.")
				return nil
			}

			if jsonOutput {
				data, jsonErr := json.MarshalIndent(result, "", "  ")
				if jsonErr != nil {
					return fmt.Errorf("JSON marshal failed: %w", jsonErr)
				}
				fmt.Println(string(data))
			} else {
				nav := sightjack.RenderNavigator(result, cfg.Linear.Project)
				fmt.Println()
				fmt.Print(nav)
			}

			// Save state
			state := &sightjack.SessionState{
				Version:         sightjack.StateFormatVersion,
				SessionID:       sessionID,
				Project:         cfg.Linear.Project,
				LastScanned:     time.Now(),
				Completeness:    result.Completeness,
				StrictnessLevel: string(cfg.Strictness.Default),
				ShibitoCount:    len(result.ShibitoWarnings),
			}
			for _, c := range result.Clusters {
				state.Clusters = append(state.Clusters, sightjack.ClusterState{
					Name:         c.Name,
					Completeness: c.Completeness,
					IssueCount:   len(c.Issues),
				})
			}

			if err := sightjack.WriteState(baseDir, state); err != nil {
				sightjack.LogWarn("Failed to save state: %v", err)
			} else {
				sightjack.LogOK("State saved to %s", sightjack.StatePath(baseDir))
			}

			sightjack.LogOK("Scan complete. Overall completeness: %.0f%%", result.Completeness*100)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Output scan result as JSON")

	return cmd
}
