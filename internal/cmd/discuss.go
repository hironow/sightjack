package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	sightjack "github.com/hironow/sightjack"
)

func newDiscussCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "discuss [path]",
		Short: "Architect discussion from stdin Wave JSON",
		Long: `Start an architect discussion about a wave from stdin.

Reads a Wave JSON (from 'select') and prompts for a discussion topic
via /dev/tty. Runs the architect agent to produce a DiscussResult
suitable for piping into 'adr' for ADR generation.`,
		Example: `  # Discuss a selected wave and generate an ADR
  sightjack select | sightjack discuss | sightjack adr > docs/adr/NNNN.md

  # Discuss with a specific project directory
  sightjack select | sightjack discuss /path/to/project`,
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
			data, err := io.ReadAll(cmd.InOrStdin())
			if err != nil {
				return fmt.Errorf("failed to read stdin: %w", err)
			}
			if len(data) == 0 {
				return fmt.Errorf("no input on stdin. Pipe a wave: sightjack select | sightjack discuss")
			}

			var wave sightjack.Wave
			if err := json.Unmarshal(data, &wave); err != nil {
				return fmt.Errorf("invalid Wave JSON: %w", err)
			}

			// Open terminal for interactive input (stdin is consumed by pipe).
			// /dev/tty is required because cmd.InOrStdin() is already exhausted
			// after io.ReadAll above — falling back to it would yield immediate EOF.
			tty, err := os.Open("/dev/tty") // nosemgrep: devtty-hard-fail-needs-fallback
			if err != nil {
				return fmt.Errorf("cannot open terminal for interactive input (stdin consumed by pipe): %w", err)
			}
			defer tty.Close()
			scanner := bufio.NewScanner(tty)

			// Prompt for discussion topic on stderr.
			errW := cmd.ErrOrStderr()
			fmt.Fprintf(errW, "\nDiscuss wave: %s - %s\n", wave.ClusterName, wave.Title)
			fmt.Fprint(errW, "Topic (or Enter to discuss the wave as-is): ")
			var topic string
			if scanner.Scan() {
				topic = strings.TrimSpace(scanner.Text())
			}
			if topic == "" {
				topic = fmt.Sprintf("Review wave %s: %s", wave.ID, wave.Title)
			}

			sessionID := fmt.Sprintf("discuss-%d-%d", time.Now().UnixMilli(), os.Getpid())
			scanDir := sightjack.ScanDir(baseDir, sessionID)
			if err := os.MkdirAll(scanDir, 0755); err != nil {
				return fmt.Errorf("failed to create scan dir: %w", err)
			}

			strictness := string(sightjack.ResolveStrictness(cfg.Strictness, []string{wave.ClusterName}))

			if dryRun {
				if err := sightjack.RunArchitectDiscussDryRun(cfg, scanDir, wave, topic, strictness); err != nil {
					return fmt.Errorf("dry-run failed: %w", err)
				}
				sightjack.LogOK("Dry-run complete. Check %s for generated prompt.", scanDir)
				return nil
			}

			resp, err := sightjack.RunArchitectDiscuss(cmd.Context(), cfg, scanDir, wave, topic, strictness)
			if err != nil {
				return fmt.Errorf("discussion failed: %w", err)
			}

			result := sightjack.ToDiscussResult(wave, resp, topic)
			out, jsonErr := json.MarshalIndent(result, "", "  ")
			if jsonErr != nil {
				return fmt.Errorf("JSON marshal failed: %w", jsonErr)
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(out))
			return nil
		},
	}
}
