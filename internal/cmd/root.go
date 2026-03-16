package cmd

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
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
	shutdownTracer func(context.Context) error
	shutdownMeter  func(context.Context) error
	finalizerOnce  sync.Once
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
		Short: "SIREN-inspired issue architecture tool for Linear",
		Long:  "sightjack — SIREN-inspired issue architecture tool for Linear\n\nClassify, wave-plan, discuss, and apply changes to Linear issues.\nRunning without a subcommand defaults to 'scan'.\nUse NeedsDefaultScan() to preprocess args before Execute.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			applyOtelEnv(filepath.Dir(cfgPath))
			noColor, _ := cmd.Flags().GetBool("no-color")
			if noColor {
				os.Setenv("NO_COLOR", "1")
			}
			logger := platform.NewLogger(cmd.ErrOrStderr(), verbose)
			logger.Header("sightjack", Version)
			logger.Section(cmd.Name())
			ctx := context.WithValue(cmd.Context(), loggerKey, logger)
			shutdownTracer = initTracer("sightjack", Version)
			shutdownMeter = initMeter("sightjack", Version)
			spanCtx := startRootSpan(ctx, cmd.Name())
			cmd.SetContext(spanCtx)
			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true, // nosemgrep: cobra-silence-errors-without-output — main.go handles error output [permanent]
	}

	finalizerOnce.Do(func() {
		cobra.OnFinalize(func() {
			endRootSpan()
			if shutdownMeter != nil {
				shutdownMeter(context.Background())
			}
			if shutdownTracer != nil {
				shutdownTracer(context.Background())
			}
		})
	})

	rootCmd.PersistentFlags().StringVarP(&cfgPath, "config", "c", ".siren/config.yaml", "Config file path")
	rootCmd.PersistentFlags().StringVarP(&lang, "lang", "l", "", "Language override (ja/en)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose logging")
	rootCmd.PersistentFlags().Bool("no-color", false, "Disable colored output (respects NO_COLOR env)")
	rootCmd.PersistentFlags().BoolVarP(&dryRun, "dry-run", "n", false, "Generate prompts without executing Claude")
	rootCmd.PersistentFlags().StringP("output", "o", "text", "Output format: text, json")

	rootCmd.Version = Version

	rootCmd.AddCommand(
		newInitCmd(),
		newDoctorCmd(),
		newShowCmd(),
		newStatusCmd(),
		newADRCmd(),
		newArchivePruneCmd(),
		newScanCmd(),
		newWavesCmd(),
		newSelectCmd(),
		newDiscussCmd(),
		newApplyCmd(),
		newNextgenCmd(),
		newRunCmd(),
		newCleanCmd(),
		newVersionCmd(),
		newUpdateCmd(),
		newConfigCmd(),
	)

	return rootCmd
}

// ttyDevices returns the ordered list of terminal device paths to try for the
// given GOOS. On Windows, CONIN$ is tried first; on Unix, /dev/tty is tried first.
func ttyDevices(goos string) []string {
	if goos == "windows" {
		return []string{"CONIN$", "/dev/tty"}
	}
	return []string{"/dev/tty", "CONIN$"}
}

// openTTY opens the platform-appropriate controlling terminal for interactive
// input. On Unix this is /dev/tty; on Windows it is CONIN$. Returns an error
// if neither device is available (e.g., in a non-interactive container).
//
// If SIGHTJACK_TTY is set, opens that path instead. This allows E2E tests
// to inject a go-expect PTY device (c.Tty().Name()) so interactive commands
// work in Docker containers without a controlling terminal.
func openTTY() (*os.File, error) {
	if path := os.Getenv("SIGHTJACK_TTY"); path != "" {
		return os.Open(path)
	}
	devices := ttyDevices(runtime.GOOS)
	var firstErr error
	for _, dev := range devices {
		tty, err := os.Open(dev)
		if err == nil {
			return tty, nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	return nil, fmt.Errorf("no controlling terminal available (tried %v: %v)", devices, firstErr)
}

// resolveBaseDir returns the absolute path from the first arg or cwd.
// Validates that the path exists and is a directory.
func resolveBaseDir(args []string) (string, error) {
	if len(args) > 0 {
		abs, err := filepath.Abs(args[0])
		if err != nil {
			return "", fmt.Errorf("resolve path: %w", err)
		}
		info, err := os.Stat(abs)
		if err != nil {
			return "", fmt.Errorf("path not found: %w", err)
		}
		if !info.IsDir() {
			return "", fmt.Errorf("not a directory: %s", abs)
		}
		return abs, nil
	}
	return os.Getwd()
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
