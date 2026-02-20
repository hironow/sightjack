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
			ctx := startSpan(cmd)
			defer endSpan(ctx)

			data, err := io.ReadAll(os.Stdin)
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

			// Open /dev/tty for interactive input (stdin is consumed by pipe).
			tty, err := os.Open("/dev/tty")
			if err != nil {
				return fmt.Errorf("cannot open /dev/tty: %w (not a terminal?)", err)
			}
			defer tty.Close()

			scanner := bufio.NewScanner(tty)

			// Prompt for discussion topic on stderr.
			fmt.Fprintf(os.Stderr, "\nDiscuss wave: %s - %s\n", wave.ClusterName, wave.Title)
			fmt.Fprint(os.Stderr, "Topic (or Enter to discuss the wave as-is): ")
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

			resp, err := sightjack.RunArchitectDiscuss(ctx, cfg, scanDir, wave, topic, strictness)
			if err != nil {
				return fmt.Errorf("discussion failed: %w", err)
			}

			result := sightjack.ToDiscussResult(wave, resp, topic)
			out, jsonErr := json.MarshalIndent(result, "", "  ")
			if jsonErr != nil {
				return fmt.Errorf("JSON marshal failed: %w", jsonErr)
			}
			fmt.Println(string(out))
			return nil
		},
	}
}
