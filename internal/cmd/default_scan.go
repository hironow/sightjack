package cmd

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// NeedsDefaultScan returns true when args contain no explicit subcommand,
// meaning "scan" should be prepended. This preserves the UX where
// `sightjack [flags]` behaves as `sightjack scan [flags]`.
func NeedsDefaultScan(rootCmd *cobra.Command, args []string) bool {
	if len(args) == 0 {
		return true
	}

	// Root-level flags that should not be redirected to scan.
	for _, arg := range args {
		if arg == "--version" || arg == "--help" || arg == "-h" {
			return false
		}
	}

	// Build set of known subcommand names.
	// Include cobra-injected commands (help, completion) that are only added
	// at Execute() time — NeedsDefaultScan runs before Execute().
	known := map[string]bool{"help": true, "completion": true}
	for _, sub := range rootCmd.Commands() {
		known[sub.Name()] = true
		for _, alias := range sub.Aliases {
			known[alias] = true
		}
	}

	// Classify persistent flags that consume a separate value arg (non-bool).
	valueTakers := make(map[string]bool)
	rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		if f.Value.Type() != "bool" {
			valueTakers["--"+f.Name] = true
			if f.Shorthand != "" {
				valueTakers["-"+f.Shorthand] = true
			}
		}
	})

	// Scan args to find a known subcommand, skipping flags and their values.
	skipNext := false
	for _, arg := range args {
		if skipNext {
			skipNext = false
			continue
		}
		if arg == "--" {
			break
		}
		if strings.HasPrefix(arg, "-") {
			if !strings.Contains(arg, "=") && valueTakers[arg] {
				skipNext = true
			}
			continue
		}
		if known[arg] {
			return false
		}
	}

	return true
}

// ReorderArgs moves a known subcommand to the front of args so cobra routes
// correctly when subcommand-local flags appear before the subcommand name
// (e.g., "sightjack --json scan" → "sightjack scan --json").
// If no subcommand is found, args are returned unchanged.
func ReorderArgs(rootCmd *cobra.Command, args []string) []string {
	if len(args) == 0 {
		return args
	}

	// Build set of known subcommand names.
	known := map[string]bool{"help": true, "completion": true}
	for _, sub := range rootCmd.Commands() {
		known[sub.Name()] = true
		for _, alias := range sub.Aliases {
			known[alias] = true
		}
	}

	// Classify persistent flags that consume a separate value arg (non-bool).
	valueTakers := make(map[string]bool)
	rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		if f.Value.Type() != "bool" {
			valueTakers["--"+f.Name] = true
			if f.Shorthand != "" {
				valueTakers["-"+f.Shorthand] = true
			}
		}
	})

	// Scan args to find a known subcommand past index 0.
	skipNext := false
	for i, arg := range args {
		if skipNext {
			skipNext = false
			continue
		}
		if arg == "--" {
			break
		}
		if strings.HasPrefix(arg, "-") {
			if !strings.Contains(arg, "=") && valueTakers[arg] {
				skipNext = true
			}
			continue
		}
		if known[arg] {
			if i == 0 {
				return args
			}
			reordered := make([]string, 0, len(args))
			reordered = append(reordered, arg)
			reordered = append(reordered, args[:i]...)
			reordered = append(reordered, args[i+1:]...)
			return reordered
		}
	}

	return args
}
