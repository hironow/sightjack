package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/spf13/cobra"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/platform"
	"github.com/hironow/sightjack/internal/session"
)

// Version, Commit, and Date are set at build time via -ldflags.
var (
	Version = "dev"
	Commit  = "dev"
	Date    = "dev"
)

type loggerKeyType struct{}

var loggerKey loggerKeyType

var (
	cfgPath string
	lang    string
	verbose bool
	dryRun  bool
)

// shutdownTracer holds the OTel tracer shutdown function registered by
// PersistentPreRunE. cobra.OnFinalize calls it after Execute completes.
var (
	shutdownTracer  func(context.Context) error
	shutdownMeter   func(context.Context) error
	sharedStreamBus interface{ Close() } // closed by OnFinalize
	finalizerOnce   sync.Once
)

func init() {
	// Ensure root PersistentPreRunE/PostRunE propagate to all subcommands.
	// Set at package init (not in NewRootCommand) to avoid re-setting on every call.
	cobra.EnableTraverseRunHooks = true
}

// NewRootCommand creates the root cobra command with all subcommands attached.
// Returning *cobra.Command enables test injection via SetArgs/SetOut/SetErr.
func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "sightjack",
		Short: "SIREN-inspired issue architecture MCP data plane",
		Long:  "sightjack — SIREN-inspired issue architecture MCP data plane\n\nServe scan/wave read models to a human-initiated claude code session via\nthe `sightjack mcp` stdio server + the /sightjack-scan skill (jun15 MCP\npivot). Use `sightjack sessions` to manage coding sessions and the\ndata-plane commands (show, status, rebuild) to inspect state.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			applyOtelEnv(filepath.Dir(cfgPath))
			noColor := mustBool(cmd, "no-color")
			if noColor {
				_ = os.Setenv("NO_COLOR", "1")
			}
			out := cmd.ErrOrStderr()
			quiet := mustBool(cmd, "quiet")
			if quiet {
				out = io.Discard
			}
			logger := platform.NewLogger(out, verbose)
			outputFmt := mustString(cmd, "output")
			if outputFmt != "json" {
				logger.Header("sightjack", Version)
				logger.Section(cmd.Name())
			}
			ctx := context.WithValue(cmd.Context(), loggerKey, logger)
			shutdownTracer = initTracer("sightjack", Version)
			shutdownMeter = initMeter("sightjack", Version)
			spanCtx := startRootSpan(ctx, cmd.Name())
			cmd.SetContext(spanCtx)

			// StreamBus: process-wide live session event bus. Closed by
			// cobra.OnFinalize. Subscribers bridge stream events to the logger.
			streamBus := platform.NewInProcessSessionBus()
			sharedStreamBus = streamBus

			// Production subscriber: bridge stream events to logger.
			sub := streamBus.Subscribe(64)
			go func() {
				for ev := range sub.C() {
					logger.Debug("stream: %s [%s] session=%s", ev.Type, ev.Tool, ev.SessionID)
				}
			}()

			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true, // nosemgrep: cobra-silence-errors-without-output — main.go handles error output [permanent]
	}

	finalizerOnce.Do(func() {
		cobra.OnFinalize(func() {
			endRootSpan()
			if sharedStreamBus != nil {
				sharedStreamBus.Close()
			}
			if shutdownMeter != nil {
				_ = shutdownMeter(context.Background())
			}
			if shutdownTracer != nil {
				_ = shutdownTracer(context.Background())
			}
		})
	})

	rootCmd.PersistentFlags().StringVarP(&cfgPath, "config", "c", ".siren/config.yaml", "Config file path")
	rootCmd.PersistentFlags().StringVarP(&lang, "lang", "l", "", "Language override (ja/en)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose logging")
	rootCmd.PersistentFlags().Bool("no-color", false, "Disable colored output (respects NO_COLOR env)")
	rootCmd.PersistentFlags().BoolP("quiet", "q", false, "Suppress all stderr output")
	rootCmd.PersistentFlags().BoolVarP(&dryRun, "dry-run", "n", false, "Generate prompts without executing Claude")
	rootCmd.PersistentFlags().StringP("output", "o", "text", "Output format: text, json")

	rootCmd.Version = Version

	rootCmd.AddCommand(
		newInitCommand(),
		newDoctorCommand(),
		newShowCommand(),
		newStatusCommand(),
		newADRCommand(),
		newArchivePruneCommand(),
		newCleanCommand(),
		newVersionCommand(),
		newUpdateCommand(),
		newConfigCommand(),
		newMCPConfigCommand(),
		newMCPCommand(),
		newSessionsCommand(),
		newRebuildCommand(),
		newDeadLettersCommand(),
		newRivalCommand(),
	)

	return rootCmd
}

// loadConfig loads the sightjack config, applying lang override if set.
func loadConfig(cmd *cobra.Command, baseDir string) (*domain.Config, error) {
	resolved := resolveConfigPath(cmd, baseDir)
	cfg, err := session.LoadConfig(resolved)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("config not found: %s\nRun 'sightjack init' to create one", resolved)
		}
		return nil, fmt.Errorf("error loading config: %w", err)
	}
	if lang != "" {
		cfg.Lang = lang
	}
	// Post jun15 MCP pivot: the headless designer pipeline is retired and
	// sightjack runs in wave mode only (Linear tracking removed). See the
	// /sightjack-scan skill + `sightjack mcp` data plane.
	cfg.Mode = domain.NewTrackingMode(false)
	return cfg, nil
}

// loggerFrom extracts the domain.Logger from the cobra command context.
// Falls back to a stderr logger if PersistentPreRunE was not executed (e.g., in tests).
func loggerFrom(cmd *cobra.Command) domain.Logger {
	if l, ok := cmd.Context().Value(loggerKey).(domain.Logger); ok {
		return l
	}
	return platform.NewLogger(cmd.ErrOrStderr(), false)
}

// resolveConfigPath returns the final config file path.
// When --config is not explicitly set, defaults to ConfigPath(baseDir).
// When explicitly set with a relative path, resolves against baseDir.
func resolveConfigPath(cmd *cobra.Command, baseDir string) string {
	if !cmd.Flags().Changed("config") {
		return domain.ConfigPath(baseDir)
	}
	if !filepath.IsAbs(cfgPath) {
		return filepath.Join(baseDir, cfgPath)
	}
	return cfgPath
}
