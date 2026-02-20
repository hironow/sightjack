package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	sightjack "github.com/hironow/sightjack"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor [path]",
		Short: "Check environment and tool availability",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			baseDir, err := resolveBaseDir(args)
			if err != nil {
				return fmt.Errorf("invalid path: %w", err)
			}
			ctx := startSpan(cmd)
			defer endSpan(ctx)

			resolved := resolveConfigPath(cmd, baseDir)

			fmt.Println("sightjack doctor — environment health check")
			fmt.Println()

			results := sightjack.RunDoctor(ctx, resolved, baseDir)

			var fails, skips int
			for _, r := range results {
				fmt.Printf("[%s] %s: %s\n", r.Status.StatusLabel(), r.Name, r.Message)
				switch r.Status {
				case sightjack.CheckFail:
					fails++
				case sightjack.CheckSkip:
					skips++
				}
			}

			fmt.Println()
			if fails == 0 && skips == 0 {
				fmt.Println("All checks passed.")
				return nil
			}
			var parts []string
			if fails > 0 {
				parts = append(parts, fmt.Sprintf("%d check(s) failed", fails))
			}
			if skips > 0 {
				parts = append(parts, fmt.Sprintf("%d skipped", skips))
			}
			fmt.Println(strings.Join(parts, ", ") + ".")
			if fails > 0 {
				return fmt.Errorf("%d check(s) failed", fails)
			}
			return nil
		},
	}
}
