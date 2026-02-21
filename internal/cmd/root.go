package cmd

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

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
var (
	shutdownTracer func(context.Context) error
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
		SilenceErrors: true, // nosemgrep: cobra-silence-errors-without-output — main.go handles error output
	}

	finalizerOnce.Do(func() {
		cobra.OnFinalize(func() {
			if shutdownTracer != nil {
				shutdownTracer(context.Background())
			}
		})
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

	// Classify persistent flags by whether they consume a separate value arg.
	valueTakers := make(map[string]bool)
	boolFlags := make(map[string]bool)
	rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		if f.Value.Type() == "bool" {
			boolFlags["--"+f.Name] = true
			if f.Shorthand != "" {
				boolFlags["-"+f.Shorthand] = true
			}
		} else {
			valueTakers["--"+f.Name] = true
			if f.Shorthand != "" {
				valueTakers["-"+f.Shorthand] = true
			}
		}
	})

	// Scan all args to find a known subcommand, skipping flags and their values.
	// We must look past unknown flags/positionals (e.g., "--json" is a scan-local
	// flag unknown to root, so "sightjack --json scan" must find "scan").
	// When found at index > 0, reorder so the subcommand comes first — cobra
	// parses flags left-to-right and would reject unknown flags before the
	// subcommand (persistent flags work anywhere, so reorder is always safe).
	skipNext := false
	skipBoolValue := false
	for i, arg := range args {
		if skipNext {
			skipNext = false
			continue
		}
		if skipBoolValue {
			skipBoolValue = false
			lower := strings.ToLower(arg)
			if lower == "true" || lower == "false" || lower == "0" || lower == "1" {
				continue
			}
		}
		if arg == "--" {
			break
		}
		if strings.HasPrefix(arg, "-") {
			// --flag=value doesn't consume the next arg.
			if !strings.Contains(arg, "=") {
				if valueTakers[arg] {
					skipNext = true
				} else if boolFlags[arg] {
					skipBoolValue = true
				}
			}
			continue
		}
		if known[arg] {
			if i == 0 {
				return args
			}
			// Move subcommand to front so cobra routes correctly.
			reordered := make([]string, 0, len(args))
			reordered = append(reordered, arg)
			reordered = append(reordered, args[:i]...)
			reordered = append(reordered, args[i+1:]...)
			return reordered
		}
		// Unknown positional — continue scanning (don't return early).
	}

	// No subcommand found → default to scan.
	return append([]string{"scan"}, args...)
}

// RewriteBoolFlags converts space-separated boolean flag values into equals
// form so pflag parses them correctly. pflag's NoOptDefVal causes --flag false
// to be parsed as --flag (true) + positional "false". This rewrites
// --flag true/false/0/1 → --flag=true/false/0/1 for all known bool flags
// across root persistent flags and all subcommand local flags.
func RewriteBoolFlags(rootCmd *cobra.Command, args []string) []string {
	boolFlags := make(map[string]bool)
	// Collect from root persistent flags.
	rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		if f.Value.Type() == "bool" {
			boolFlags["--"+f.Name] = true
			if f.Shorthand != "" {
				boolFlags["-"+f.Shorthand] = true
			}
		}
	})
	// Collect from all subcommand local flags.
	for _, sub := range rootCmd.Commands() {
		sub.LocalFlags().VisitAll(func(f *pflag.Flag) {
			if f.Value.Type() == "bool" {
				boolFlags["--"+f.Name] = true
				if f.Shorthand != "" {
					boolFlags["-"+f.Shorthand] = true
				}
			}
		})
	}

	result := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			result = append(result, args[i:]...)
			break
		}
		if strings.Contains(arg, "=") || !strings.HasPrefix(arg, "-") {
			result = append(result, arg)
			continue
		}
		if boolFlags[arg] && i+1 < len(args) {
			next := strings.ToLower(args[i+1])
			if next == "true" || next == "false" || next == "0" || next == "1" {
				result = append(result, arg+"="+args[i+1])
				i++ // consume next
				continue
			}
		}
		result = append(result, arg)
	}
	return result
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
