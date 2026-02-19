package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	sightjack "github.com/hironow/sightjack"
)

var version = "0.0.12"

func main() {
	// Extract subcommand and optional path before flag parsing so flags
	// after the subcommand are honored.
	// e.g. "sightjack scan --dry-run" and "sightjack --dry-run scan" both work.
	subcmd, repoPath, flagArgs, extractErr := extractSubcommand(os.Args[1:])
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
	setUsage(fs)
	fs.StringVar(&configPath, "config", ".siren/config.yaml", "Config file path")
	fs.StringVar(&configPath, "c", ".siren/config.yaml", "Config file path (shorthand)")
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
		fmt.Fprintf(os.Stderr, "unexpected argument: %s\nUsage: sightjack [scan|show|session|init|doctor] [flags] [path]\n", fs.Arg(0))
		os.Exit(1)
	}

	// Resolve baseDir from path argument or cwd.
	var baseDir string
	if repoPath != "" {
		absPath, err := filepath.Abs(repoPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid path: %v\n", err)
			os.Exit(1)
		}
		baseDir = absPath
	} else {
		wd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to get working directory: %v\n", err)
			os.Exit(1)
		}
		baseDir = wd
	}

	configPath = resolveConfigPath(configPath, baseDir, configExplicitlySet(fs))

	sightjack.SetVerbose(verbose)

	// Initialize OpenTelemetry tracer (noop when OTEL_EXPORTER_OTLP_ENDPOINT is unset).
	shutdownTracer := sightjack.InitTracer("sightjack", version)
	defer shutdownTracer(context.Background())

	switch subcmd {
	case "scan":
		cfg := loadConfigOrExit(configPath)
		if lang != "" {
			cfg.Lang = lang
		}
		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()
		ctx = sightjack.StartRootSpan(ctx, subcmd)
		runScan(ctx, cfg, baseDir, dryRun)
		sightjack.EndRootSpan(ctx)
	case "session":
		cfg := loadConfigOrExit(configPath)
		if lang != "" {
			cfg.Lang = lang
		}
		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()
		ctx = sightjack.StartRootSpan(ctx, subcmd)

		// Check for existing state (resume detection)
		if !dryRun {
			existingState, stateErr := sightjack.ReadState(baseDir)
			if stateErr != nil {
				// Try recovery from cached scan results.
				// Session directories are named "session-{unixmilli}-{pid}", so
				// lexicographic order matches creation time. Iterate in descending
				// order and use the first directory with recoverable data, so a
				// crashed/corrupt newest session doesn't block recovery from older ones.
				scansDir := filepath.Join(baseDir, ".siren", ".run")
				entries, dirErr := os.ReadDir(scansDir)
				if dirErr == nil && len(entries) > 0 {
					for i := len(entries) - 1; i >= 0; i-- {
						entry := entries[i]
						if !entry.IsDir() {
							continue
						}
						recovered, recErr := sightjack.TryRecoverState(baseDir, entry.Name())
						if recErr == nil {
							existingState = recovered
							stateErr = nil
							break
						}
					}
				}
			}
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
						sightjack.EndRootSpan(ctx)
						return
					case sightjack.ResumeChoiceRescan:
						if err := sightjack.RunRescanSession(ctx, cfg, baseDir, existingState, os.Stdin); err != nil {
							sightjack.LogError("Re-scan resume failed: %v", err)
							os.Exit(1)
						}
						sightjack.EndRootSpan(ctx)
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
		sightjack.EndRootSpan(ctx)
	case "show":
		ctx := sightjack.StartRootSpan(context.Background(), subcmd)
		runShow(baseDir)
		sightjack.EndRootSpan(ctx)
	case "init":
		ctx := sightjack.StartRootSpan(context.Background(), subcmd)
		if err := runInit(baseDir, os.Stdin, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "init failed: %v\n", err)
			os.Exit(1)
		}
		sightjack.EndRootSpan(ctx)
	case "doctor":
		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()
		ctx = sightjack.StartRootSpan(ctx, subcmd)
		runDoctor(ctx, configPath, baseDir)
		sightjack.EndRootSpan(ctx)
	}
}

// setUsage configures a custom usage function for the FlagSet that shows
// available subcommands alongside the standard flag defaults.
func setUsage(fs *flag.FlagSet) {
	fs.Usage = func() {
		out := fs.Output()
		fmt.Fprintf(out, "sightjack — SIREN-inspired issue architecture tool for Linear\n\n")
		fmt.Fprintf(out, "Usage: sightjack [command] [flags] [path]\n\n")
		fmt.Fprintf(out, "Commands:\n")
		fmt.Fprintf(out, "  scan      Classify and deep-scan Linear issues (default)\n")
		fmt.Fprintf(out, "  session   Interactive wave approval and apply session\n")
		fmt.Fprintf(out, "  show      Display last scan results\n")
		fmt.Fprintf(out, "  init      Create .siren/config.yaml interactively\n")
		fmt.Fprintf(out, "  doctor    Check environment and tool availability\n\n")
		fmt.Fprintf(out, "Flags:\n")
		fs.PrintDefaults()
	}
}

// configExplicitlySet returns true if -c or --config was explicitly passed
// on the command line (as opposed to using the default value).
// resolveConfigPath returns the final config path.
// When not explicitly set, it defaults to ConfigPath(baseDir).
// When explicitly set with a relative path, it resolves against baseDir.
func resolveConfigPath(configPath, baseDir string, explicitlySet bool) string {
	if !explicitlySet {
		return sightjack.ConfigPath(baseDir)
	}
	if !filepath.IsAbs(configPath) {
		return filepath.Join(baseDir, configPath)
	}
	return configPath
}

func configExplicitlySet(fs *flag.FlagSet) bool {
	explicit := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "config" || f.Name == "c" {
			explicit = true
		}
	})
	return explicit
}

// extractSubcommand finds the first known subcommand and an optional path
// argument in args, returning them along with the remaining flags.
// A non-flag positional that isn't a known command is treated as a path.
// At most one path is allowed; a second non-command positional is an error.
// Correctly skips flag values so that e.g. "-c custom.yaml scan" works.
func extractSubcommand(args []string) (string, string, []string, error) {
	knownCmds := map[string]bool{"scan": true, "show": true, "session": true, "init": true, "doctor": true}
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
	var path string
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
				return "", "", nil, fmt.Errorf("unexpected argument: %s\nUsage: sightjack [scan|show|session|init|doctor] [flags] [path]", arg)
			}
			subcmd = arg
			continue
		}
		// Non-flag, non-command positional — treat as path.
		if path != "" {
			return "", "", nil, fmt.Errorf("unexpected argument: %s\nOnly one path argument is allowed.", arg)
		}
		path = arg
	}
	if subcmd == "" {
		subcmd = "scan"
	}
	return subcmd, path, filtered, nil
}

// loadConfigOrExit loads the config file and exits with a helpful message on failure.
func loadConfigOrExit(configPath string) *sightjack.Config {
	cfg, err := sightjack.LoadConfig(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "Config not found: %s\nRun 'sightjack init' to create one.\n", configPath)
		} else {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		}
		os.Exit(1)
	}
	return cfg
}

func runScan(ctx context.Context, cfg *sightjack.Config, baseDir string, dryRun bool) {
	sessionID := fmt.Sprintf("scan-%d-%d", time.Now().UnixMilli(), os.Getpid())

	sightjack.LogInfo("Starting sightjack scan...")
	sightjack.LogInfo("Team: %s | Project: %s | Lang: %s", cfg.Linear.Team, cfg.Linear.Project, cfg.Lang)

	result, err := sightjack.RunScan(ctx, cfg, baseDir, sessionID, dryRun)
	if err != nil {
		sightjack.LogError("Scan failed: %v", err)
		os.Exit(1)
	}

	if dryRun {
		sightjack.LogOK("Dry-run complete. Check .siren/.run/ for generated prompts.")
		return
	}

	// Display Link Navigator
	nav := sightjack.RenderNavigator(result, cfg.Linear.Project)
	fmt.Println()
	fmt.Print(nav)

	// Save state
	state := &sightjack.SessionState{
		Version:         sightjack.StateFormatVersion,
		SessionID:       sessionID,
		Project:         cfg.Linear.Project,
		LastScanned:     time.Now(),
		Completeness:    result.Completeness,
		StrictnessLevel: string(cfg.Strictness.Default),
		ShibitoCount:    len(result.ShibitoWarnings),
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

func runShow(baseDir string) {
	state, err := sightjack.ReadState(baseDir)
	if err != nil {
		sightjack.LogError("No previous scan found: %v", err)
		sightjack.LogInfo("Run 'sightjack scan' first.")
		os.Exit(1)
	}

	// Reconstruct ScanResult from state
	result := &sightjack.ScanResult{
		Completeness: state.Completeness,
	}
	for _, c := range state.Clusters {
		result.Clusters = append(result.Clusters, sightjack.ClusterScanResult{
			Name:         c.Name,
			Completeness: c.Completeness,
			IssueCount:   c.IssueCount,
		})
		result.TotalIssues += c.IssueCount
	}

	// Restore waves from state for matrix navigator
	waves := sightjack.RestoreWaves(state.Waves)

	strictness := state.StrictnessLevel
	if strictness == "" {
		strictness = "fog" // backward compat: older state files lack this field
	}
	// show is not a resume flow — pass nil for lastScanned to suppress "Session: resumed" banner
	nav := sightjack.RenderMatrixNavigator(result, state.Project, waves, state.ADRCount, nil, strictness, state.ShibitoCount)
	fmt.Println()
	fmt.Print(nav)
	sightjack.LogInfo("Last scanned: %s", state.LastScanned.Format("2006-01-02 15:04:05"))
}

// runInit creates .siren/config.yaml interactively by reading from r and
// writing prompts to w. Returns an error if the config file already exists.
func runInit(baseDir string, r io.Reader, w io.Writer) error {
	cfgPath := sightjack.ConfigPath(baseDir)
	if _, err := os.Stat(cfgPath); err == nil {
		return fmt.Errorf(".siren/config.yaml already exists in %s", baseDir)
	}

	scanner := bufio.NewScanner(r)

	fmt.Fprintln(w, "sightjack init — create .siren/config.yaml")
	fmt.Fprintln(w)

	// team (required)
	var team string
	for team == "" {
		fmt.Fprint(w, "Linear team name: ")
		if !scanner.Scan() {
			return fmt.Errorf("unexpected end of input")
		}
		team = strings.TrimSpace(scanner.Text())
	}

	// project (required)
	var project string
	for project == "" {
		fmt.Fprint(w, "Linear project name: ")
		if !scanner.Scan() {
			return fmt.Errorf("unexpected end of input")
		}
		project = strings.TrimSpace(scanner.Text())
	}

	// lang (default: ja)
	lang := "ja"
	for {
		fmt.Fprint(w, "Language (ja/en) [ja]: ")
		if !scanner.Scan() {
			break
		}
		v := strings.TrimSpace(scanner.Text())
		if v == "" {
			break // keep default
		}
		if sightjack.ValidLang(v) {
			lang = v
			break
		}
		fmt.Fprintf(w, "  invalid language %q (valid: ja, en)\n", v)
	}

	// strictness (default: fog)
	strictness := "fog"
	for {
		fmt.Fprint(w, "Strictness (fog/alert/lockdown) [fog]: ")
		if !scanner.Scan() {
			break
		}
		v := strings.TrimSpace(scanner.Text())
		if v == "" {
			break // keep default
		}
		if _, err := sightjack.ParseStrictnessLevel(v); err == nil {
			strictness = strings.ToLower(v)
			break
		}
		fmt.Fprintf(w, "  invalid strictness %q (valid: fog, alert, lockdown)\n", v)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read input: %w", err)
	}

	// Ensure .siren/ directory exists
	sirenDir := filepath.Join(baseDir, ".siren")
	if err := os.MkdirAll(sirenDir, 0755); err != nil {
		return fmt.Errorf("create .siren dir: %w", err)
	}

	content := sightjack.RenderInitConfig(team, project, lang, strictness)
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	// Write .gitignore (best-effort)
	_ = sightjack.WriteGitIgnore(baseDir)

	fmt.Fprintln(w)
	fmt.Fprintf(w, "Created .siren/config.yaml\n")
	return nil
}

func runDoctor(ctx context.Context, configPath string, baseDir string) {
	fmt.Println("sightjack doctor — environment health check")
	fmt.Println()

	results := sightjack.RunDoctor(ctx, configPath, baseDir)

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
	} else {
		parts := []string{}
		if fails > 0 {
			parts = append(parts, fmt.Sprintf("%d check(s) failed", fails))
		}
		if skips > 0 {
			parts = append(parts, fmt.Sprintf("%d skipped", skips))
		}
		fmt.Println(strings.Join(parts, ", ") + ".")
		if fails > 0 {
			os.Exit(1)
		}
	}
}
