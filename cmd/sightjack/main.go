package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	sightjack "github.com/hironow/sightjack"
)

var version = "0.8.0-dev"

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
		fmt.Fprintf(os.Stderr, "unexpected argument: %s\nUsage: sightjack [scan|show|session] [flags]\n", fs.Arg(0))
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
	case "session":
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
		baseDir, err := os.Getwd()
		if err != nil {
			sightjack.LogError("Failed to get working directory: %v", err)
			os.Exit(1)
		}

		// Check for existing state (resume detection)
		if !dryRun {
			existingState, stateErr := sightjack.ReadState(baseDir)
			if stateErr == nil {
				scanner := bufio.NewScanner(os.Stdin)
				for {
					choice, promptErr := sightjack.PromptResume(ctx, os.Stdout, scanner, existingState)
					if promptErr == sightjack.ErrQuit {
						return
					}
					if promptErr != nil {
						sightjack.LogWarn("Invalid input: %v", promptErr)
						continue // re-prompt instead of falling through
					}
					switch choice {
					case sightjack.ResumeChoiceResume:
						if !sightjack.CanResume(existingState) {
							sightjack.LogWarn("Cached scan data missing — starting fresh session instead.")
							goto freshSession
						}
						if err := sightjack.RunResumeSession(ctx, cfg, baseDir, existingState, os.Stdin); err != nil {
							sightjack.LogError("Resume failed: %v", err)
							os.Exit(1)
						}
						return
					case sightjack.ResumeChoiceRescan:
						if err := sightjack.RunRescanSession(ctx, cfg, baseDir, existingState, os.Stdin); err != nil {
							sightjack.LogError("Re-scan resume failed: %v", err)
							os.Exit(1)
						}
						return
					case sightjack.ResumeChoiceNew:
						goto freshSession
					}
				}
			}
		}
	freshSession:

		// Fresh session
		sessionID := fmt.Sprintf("session-%d-%d", time.Now().UnixMilli(), os.Getpid())
		var sessionInput io.Reader
		if !dryRun {
			sessionInput = os.Stdin
		}
		if err := sightjack.RunSession(ctx, cfg, baseDir, sessionID, dryRun, sessionInput); err != nil {
			sightjack.LogError("Session failed: %v", err)
			os.Exit(1)
		}
	case "show":
		runShow()
	}
}

// extractSubcommand finds the first known subcommand in args, returning it and the remaining flags.
// Returns an error if a non-flag positional argument is found that isn't a known command.
// Correctly skips flag values so that e.g. "-c custom.yaml scan" works.
func extractSubcommand(args []string) (string, []string, error) {
	knownCmds := map[string]bool{"scan": true, "show": true, "session": true}
	// Flags that consume the next token as their value.
	valuedFlags := map[string]bool{
		"-config": true, "--config": true, "-c": true,
		"-lang": true, "--lang": true, "-l": true,
	}
	// Boolean flags that may optionally consume "true"/"false" as next token.
	boolFlags := map[string]bool{
		"-verbose": true, "--verbose": true, "-v": true,
		"-dry-run": true, "--dry-run": true,
		"-version": true, "--version": true,
	}

	var subcmd string
	var filtered []string
	skipNext := false
	lastBoolFlag := "" // non-empty when the previous token was a boolean flag

	for _, arg := range args {
		if skipNext {
			skipNext = false
			filtered = append(filtered, arg)
			continue
		}
		// After a boolean flag, merge "true"/"false" into --flag=value form.
		if lastBoolFlag != "" {
			flag := lastBoolFlag
			lastBoolFlag = ""
			lower := strings.ToLower(arg)
			if lower == "true" || lower == "false" {
				filtered[len(filtered)-1] = flag + "=" + lower
				continue
			}
		}
		if strings.HasPrefix(arg, "-") {
			filtered = append(filtered, arg)
			if strings.Contains(arg, "=") {
				continue
			}
			if valuedFlags[arg] {
				skipNext = true
			} else if boolFlags[arg] {
				lastBoolFlag = arg
			}
			continue
		}
		if knownCmds[arg] {
			if subcmd != "" {
				return "", nil, fmt.Errorf("unexpected argument: %s\nUsage: sightjack [scan|show|session] [flags]", arg)
			}
			subcmd = arg
			continue
		}
		return "", nil, fmt.Errorf("unknown command: %s\nUsage: sightjack [scan|show|session]", arg)
	}
	if subcmd == "" {
		subcmd = "scan"
	}
	return subcmd, filtered, nil
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
