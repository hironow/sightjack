package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	sightjack "github.com/hironow/sightjack"
)

// version, commit, date are set by -ldflags at build time (GoReleaser).
// Defaults to "dev" for local development (go run / go build without flags).
var (
	version = "dev"
	commit  = "dev"
	date    = "dev"
)

var (
	cfgPath string
	lang    string
	verbose bool
	dryRun  bool
)

// shutdownTracer holds the OTel tracer shutdown function registered by
// PersistentPreRunE. cobra.OnFinalize calls it after Execute completes.
var shutdownTracer func(context.Context) error

// NewRootCommand creates the root cobra command with all subcommands attached.
// Returning *cobra.Command enables test injection via SetArgs/SetOut/SetErr.
func NewRootCommand() *cobra.Command {
	cobra.EnableTraverseRunHooks = true

	rootCmd := &cobra.Command{
		Use:   "sightjack",
		Short: "SIREN-inspired issue architecture tool for Linear",
		Long:  "sightjack — SIREN-inspired issue architecture tool for Linear\n\nClassify, wave-plan, discuss, and apply changes to Linear issues.\nRunning without a subcommand defaults to 'scan'.\nUse DefaultToScan() to preprocess args before Execute.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			sightjack.SetVerbose(verbose)
			shutdownTracer = sightjack.InitTracer("sightjack", version)
			spanCtx := sightjack.StartRootSpan(cmd.Context(), cmd.Name())
			cmd.SetContext(spanCtx)
			return nil
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			sightjack.EndRootSpan(cmd.Context())
			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cobra.OnFinalize(func() {
		if shutdownTracer != nil {
			shutdownTracer(context.Background())
		}
	})

	rootCmd.PersistentFlags().StringVarP(&cfgPath, "config", "c", ".siren/config.yaml", "Config file path")
	rootCmd.PersistentFlags().StringVarP(&lang, "lang", "l", "", "Language override (ja/en)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose logging")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Generate prompts without executing Claude")

	rootCmd.Version = version

	rootCmd.AddCommand(
		newInitCmd(),
		newDoctorCmd(),
		newShowCmd(),
		newADRCmd(),
		newArchivePruneCmd(),
		newScanCmd(),
		newWavesCmd(),
		newSelectCmd(),
		newDiscussCmd(),
		newApplyCmd(),
		newNextgenCmd(),
		newRunCmd(),
		newVersionCmd(),
		newUpdateCmd(),
	)

	return rootCmd
}

// DefaultToScan preprocesses CLI args to inject "scan" when no subcommand
// is detected. This preserves pre-cobra behavior where flags like --json
// are forwarded to the scan command. Call before rootCmd.ExecuteContext.
func DefaultToScan(rootCmd *cobra.Command, args []string) []string {
	if len(args) == 0 {
		return []string{"scan"}
	}

	// Root-level flags that should not be redirected to scan.
	for _, arg := range args {
		if arg == "--version" || arg == "--help" || arg == "-h" {
			return args
		}
	}

	// Build set of known subcommand names.
	known := make(map[string]bool)
	for _, sub := range rootCmd.Commands() {
		known[sub.Name()] = true
		for _, alias := range sub.Aliases {
			known[alias] = true
		}
	}

	// If the first positional argument (non-flag) is a known subcommand,
	// leave args unchanged. Otherwise prepend "scan".
	for _, arg := range args {
		if arg == "--" {
			break
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		if known[arg] {
			return args
		}
		return append([]string{"scan"}, args...)
	}

	// All args are flags (e.g., "--json") → default to scan.
	return append([]string{"scan"}, args...)
}

// resolveBaseDir returns the absolute path from the first arg or cwd.
func resolveBaseDir(args []string) (string, error) {
	if len(args) > 0 {
		return filepath.Abs(args[0])
	}
	return os.Getwd()
}

// loadConfig loads the sightjack config, applying lang override if set.
func loadConfig(cmd *cobra.Command, baseDir string) (*sightjack.Config, error) {
	resolved := resolveConfigPath(cmd, baseDir)
	cfg, err := sightjack.LoadConfig(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config not found: %s\nRun 'sightjack init' to create one", resolved)
		}
		return nil, fmt.Errorf("error loading config: %w", err)
	}
	if lang != "" {
		cfg.Lang = lang
	}
	return cfg, nil
}

// resolveConfigPath returns the final config file path.
// When --config is not explicitly set, defaults to ConfigPath(baseDir).
// When explicitly set with a relative path, resolves against baseDir.
func resolveConfigPath(cmd *cobra.Command, baseDir string) string {
	if !cmd.Flags().Changed("config") {
		return sightjack.ConfigPath(baseDir)
	}
	if !filepath.IsAbs(cfgPath) {
		return filepath.Join(baseDir, cfgPath)
	}
	return cfgPath
}
