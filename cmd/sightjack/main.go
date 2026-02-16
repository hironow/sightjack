package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	sightjack "github.com/hironow/sightjack"
)

var version = "0.1.0-dev"

func main() {
	// Extract subcommand before flag parsing so flags after the subcommand are honored.
	// e.g. "sightjack scan --dry-run" and "sightjack --dry-run scan" both work.
	subcmd, flagArgs, extractErr := extractSubcommand(os.Args[1:])
	if extractErr != nil {
		fmt.Fprintln(os.Stderr, extractErr)
		os.Exit(1)
	}

	var (
		configPath string
		lang       string
		verbose    bool
		dryRun     bool
		showVer    bool
	)

	fs := flag.NewFlagSet("sightjack", flag.ExitOnError)
	fs.StringVar(&configPath, "config", "sightjack.yaml", "Config file path")
	fs.StringVar(&configPath, "c", "sightjack.yaml", "Config file path (shorthand)")
	fs.StringVar(&lang, "lang", "", "Language override (ja/en)")
	fs.StringVar(&lang, "l", "", "Language override (shorthand)")
	fs.BoolVar(&verbose, "verbose", false, "Verbose logging")
	fs.BoolVar(&verbose, "v", false, "Verbose logging (shorthand)")
	fs.BoolVar(&dryRun, "dry-run", false, "Generate prompts without executing Claude")
	fs.BoolVar(&showVer, "version", false, "Show version")
	fs.Parse(flagArgs)

	if showVer {
		fmt.Printf("sightjack %s\n", version)
		os.Exit(0)
	}

	if fs.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "unexpected argument: %s\nUsage: sightjack [scan|show] [flags]\n", fs.Arg(0))
		os.Exit(1)
	}

	sightjack.SetVerbose(verbose)

	switch subcmd {
	case "scan":
		cfg, err := sightjack.LoadConfig(configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}
		if lang != "" {
			cfg.Lang = lang
		}
		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()
		runScan(ctx, cfg, dryRun)
	case "show":
		runShow()
	}
}

// extractSubcommand finds the first known subcommand in args, returning it and the remaining flags.
// Returns an error if a non-flag positional argument is found that isn't a known command.
// Correctly skips flag values so that e.g. "-c custom.yaml scan" works.
func extractSubcommand(args []string) (string, []string, error) {
	knownCmds := map[string]bool{"scan": true, "show": true}
	// Flags that consume the next token as their value.
	valuedFlags := map[string]bool{
		"-config": true, "--config": true, "-c": true,
		"-lang": true, "--lang": true, "-l": true,
	}

	skipNext := false
	for i, arg := range args {
		if skipNext {
			skipNext = false
			continue
		}
		if strings.HasPrefix(arg, "-") {
			// Flag with embedded value (--flag=val) is self-contained.
			if strings.Contains(arg, "=") {
				continue
			}
			if valuedFlags[arg] {
				skipNext = true
			}
			continue
		}
		if knownCmds[arg] {
			return arg, append(args[:i], args[i+1:]...), nil
		}
		return "", nil, fmt.Errorf("unknown command: %s\nUsage: sightjack [scan|show]", arg)
	}
	return "scan", args, nil
}

func runScan(ctx context.Context, cfg *sightjack.Config, dryRun bool) {
	baseDir, err := os.Getwd()
	if err != nil {
		sightjack.LogError("Failed to get working directory: %v", err)
		os.Exit(1)
	}

	sessionID := fmt.Sprintf("scan-%d-%d", time.Now().UnixMilli(), os.Getpid())

	sightjack.LogInfo("Starting sightjack scan...")
	sightjack.LogInfo("Team: %s | Project: %s | Lang: %s", cfg.Linear.Team, cfg.Linear.Project, cfg.Lang)

	result, err := sightjack.RunScan(ctx, cfg, baseDir, sessionID, dryRun)
	if err != nil {
		sightjack.LogError("Scan failed: %v", err)
		os.Exit(1)
	}

	if dryRun {
		sightjack.LogOK("Dry-run complete. Check .siren/scans/ for generated prompts.")
		return
	}

	// Display Link Navigator
	nav := sightjack.RenderNavigator(result, cfg.Linear.Project)
	fmt.Println()
	fmt.Print(nav)

	// Save state
	state := &sightjack.SessionState{
		Version:      "0.1",
		SessionID:    sessionID,
		Project:      cfg.Linear.Project,
		LastScanned:  time.Now(),
		Completeness: result.Completeness,
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
}

func runShow() {
	baseDir, err := os.Getwd()
	if err != nil {
		sightjack.LogError("Failed to get working directory: %v", err)
		os.Exit(1)
	}

	state, err := sightjack.ReadState(baseDir)
	if err != nil {
		sightjack.LogError("No previous scan found: %v", err)
		sightjack.LogInfo("Run 'sightjack scan' first.")
		os.Exit(1)
	}

	// Reconstruct minimal ScanResult from state
	result := &sightjack.ScanResult{
		Completeness: state.Completeness,
	}
	for _, c := range state.Clusters {
		result.Clusters = append(result.Clusters, sightjack.ClusterScanResult{
			Name:         c.Name,
			Completeness: c.Completeness,
		})
		result.TotalIssues += c.IssueCount
	}

	nav := sightjack.RenderNavigator(result, state.Project)
	fmt.Println()
	fmt.Print(nav)
	sightjack.LogInfo("Last scanned: %s", state.LastScanned.Format("2006-01-02 15:04:05"))
}
