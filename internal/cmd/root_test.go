package cmd

import (
	"testing"

	"github.com/spf13/cobra"

	sightjack "github.com/hironow/sightjack"
)

// newTestCmd creates a cobra.Command with --config flag registered on Flags()
// for direct unit testing of resolveConfigPath (without cobra's execution-time
// persistent flag merging).
func newTestCmd() *cobra.Command {
	c := &cobra.Command{}
	c.Flags().StringVarP(&cfgPath, "config", "c", ".siren/config.yaml", "Config file path")
	return c
}

func TestResolveConfigPath_RelativeWithBaseDir(t *testing.T) {
	// given: command with --config explicitly set to a relative path
	c := newTestCmd()
	c.Flags().Set("config", ".siren/config.yaml")

	// when
	got := resolveConfigPath(c, "/repo")

	// then: should resolve relative to baseDir
	want := "/repo/.siren/config.yaml"
	if got != want {
		t.Errorf("resolveConfigPath: expected %q, got %q", want, got)
	}
}

func TestResolveConfigPath_AbsoluteUnchanged(t *testing.T) {
	// given: command with --config explicitly set to an absolute path
	c := newTestCmd()
	c.Flags().Set("config", "/custom/config.yaml")

	// when
	got := resolveConfigPath(c, "/repo")

	// then: should remain unchanged
	want := "/custom/config.yaml"
	if got != want {
		t.Errorf("resolveConfigPath: expected %q, got %q", want, got)
	}
}

func TestResolveConfigPath_NotExplicit_UsesDefault(t *testing.T) {
	// given: command without --config explicitly set
	c := newTestCmd()

	// when
	got := resolveConfigPath(c, "/repo")

	// then: should use ConfigPath(baseDir)
	want := sightjack.ConfigPath("/repo")
	if got != want {
		t.Errorf("resolveConfigPath: expected %q, got %q", want, got)
	}
}

func TestDefaultToScan_NoArgs(t *testing.T) {
	// given: no args at all — should default to scan
	rootCmd := NewRootCommand()

	// when
	got := DefaultToScan(rootCmd, []string{})

	// then
	if len(got) != 1 || got[0] != "scan" {
		t.Errorf("expected [scan], got %v", got)
	}
}

func TestDefaultToScan_PathOnly(t *testing.T) {
	// given: just a path — should prepend scan
	rootCmd := NewRootCommand()

	// when
	got := DefaultToScan(rootCmd, []string{"."})

	// then
	if len(got) != 2 || got[0] != "scan" || got[1] != "." {
		t.Errorf("expected [scan .], got %v", got)
	}
}

func TestDefaultToScan_ScanLocalFlag(t *testing.T) {
	// given: --json flag (scan-local) with path — should prepend scan
	rootCmd := NewRootCommand()

	// when
	got := DefaultToScan(rootCmd, []string{"--json", "."})

	// then
	if len(got) != 3 || got[0] != "scan" {
		t.Errorf("expected [scan --json .], got %v", got)
	}
}

func TestDefaultToScan_FlagOnly(t *testing.T) {
	// given: only flags, no positional args — should prepend scan
	rootCmd := NewRootCommand()

	// when
	got := DefaultToScan(rootCmd, []string{"--json"})

	// then
	if len(got) != 2 || got[0] != "scan" {
		t.Errorf("expected [scan --json], got %v", got)
	}
}

func TestDefaultToScan_ExplicitSubcommand(t *testing.T) {
	// given: explicit subcommand — should not change
	rootCmd := NewRootCommand()

	// when
	got := DefaultToScan(rootCmd, []string{"version", "--json"})

	// then
	if len(got) != 2 || got[0] != "version" {
		t.Errorf("expected [version --json] unchanged, got %v", got)
	}
}

func TestDefaultToScan_VersionFlag(t *testing.T) {
	// given: --version flag — should not redirect to scan
	rootCmd := NewRootCommand()

	// when
	got := DefaultToScan(rootCmd, []string{"--version"})

	// then
	if len(got) != 1 || got[0] != "--version" {
		t.Errorf("expected [--version] unchanged, got %v", got)
	}
}

func TestDefaultToScan_HelpFlag(t *testing.T) {
	// given: --help flag — should not redirect to scan
	rootCmd := NewRootCommand()

	// when
	got := DefaultToScan(rootCmd, []string{"--help"})

	// then
	if len(got) != 1 || got[0] != "--help" {
		t.Errorf("expected [--help] unchanged, got %v", got)
	}
}

func TestDefaultToScan_PersistentFlagBeforeSubcommand(t *testing.T) {
	// given: --verbose (persistent boolean) before explicit subcommand
	rootCmd := NewRootCommand()

	// when
	got := DefaultToScan(rootCmd, []string{"--verbose", "doctor", "."})

	// then: "doctor" reordered to front (persistent flags work after subcommand)
	if len(got) != 3 || got[0] != "doctor" {
		t.Errorf("expected [doctor --verbose .], got %v", got)
	}
}

func TestDefaultToScan_ValueFlagBeforeSubcommand(t *testing.T) {
	// given: --config takes a value — "custom.yaml" must be skipped, "waves" found
	rootCmd := NewRootCommand()

	// when
	got := DefaultToScan(rootCmd, []string{"--config", "custom.yaml", "waves"})

	// then: "waves" reordered to front
	if len(got) != 3 || got[0] != "waves" {
		t.Errorf("expected [waves --config custom.yaml], got %v", got)
	}
}

func TestDefaultToScan_ShortValueFlagBeforeSubcommand(t *testing.T) {
	// given: -c (shorthand for --config) takes a value
	rootCmd := NewRootCommand()

	// when
	got := DefaultToScan(rootCmd, []string{"-c", "custom.yaml", "doctor"})

	// then: "doctor" reordered to front
	if len(got) != 3 || got[0] != "doctor" {
		t.Errorf("expected [doctor -c custom.yaml], got %v", got)
	}
}

func TestDefaultToScan_ValueFlagEqualsForm(t *testing.T) {
	// given: --config=custom.yaml (equals form) — no separate value arg
	rootCmd := NewRootCommand()

	// when
	got := DefaultToScan(rootCmd, []string{"--config=custom.yaml", "waves"})

	// then: "waves" reordered to front
	if len(got) != 2 || got[0] != "waves" {
		t.Errorf("expected [waves --config=custom.yaml], got %v", got)
	}
}

func TestDefaultToScan_ValueFlagBeforePath(t *testing.T) {
	// given: --lang ja /path — /path is not a subcommand, should prepend scan
	rootCmd := NewRootCommand()

	// when
	got := DefaultToScan(rootCmd, []string{"--lang", "ja", "/some/path"})

	// then: should prepend scan
	if len(got) != 4 || got[0] != "scan" {
		t.Errorf("expected [scan --lang ja /some/path], got %v", got)
	}
}

func TestDefaultToScan_BoolFlagExplicitValueBeforeSubcommand(t *testing.T) {
	// given: --dry-run false waves — "false" is a bool value, not a path
	rootCmd := NewRootCommand()

	// when
	got := DefaultToScan(rootCmd, []string{"--dry-run", "false", "waves"})

	// then: "waves" reordered to front
	if len(got) != 3 || got[0] != "waves" {
		t.Errorf("expected [waves --dry-run false], got %v", got)
	}
}

func TestDefaultToScan_BoolFlagExplicitTrueBeforeSubcommand(t *testing.T) {
	// given: --verbose true doctor — "true" is a bool value, not a path
	rootCmd := NewRootCommand()

	// when
	got := DefaultToScan(rootCmd, []string{"--verbose", "true", "doctor"})

	// then: "doctor" reordered to front
	if len(got) != 3 || got[0] != "doctor" {
		t.Errorf("expected [doctor --verbose true], got %v", got)
	}
}

func TestDefaultToScan_BoolFlagNonBoolValueTreatedAsPositional(t *testing.T) {
	// given: --verbose mypath — "mypath" is NOT a bool value, treat as positional
	rootCmd := NewRootCommand()

	// when
	got := DefaultToScan(rootCmd, []string{"--verbose", "mypath"})

	// then: should prepend scan — "mypath" is a path, not a bool value
	if len(got) != 3 || got[0] != "scan" {
		t.Errorf("expected [scan --verbose mypath], got %v", got)
	}
}

func TestDefaultToScan_ScanLocalFlagBeforeSubcommand(t *testing.T) {
	// given: --json scan — --json is a scan-local flag unknown to root,
	// "scan" is a known subcommand that must be found and reordered to front.
	rootCmd := NewRootCommand()

	// when
	got := DefaultToScan(rootCmd, []string{"--json", "scan"})

	// then: "scan" reordered to front so cobra routes correctly
	if len(got) != 2 || got[0] != "scan" || got[1] != "--json" {
		t.Errorf("expected [scan --json], got %v", got)
	}
}

func TestDefaultToScan_UnknownFlagBeforeSubcommand(t *testing.T) {
	// given: --days 7 archive-prune — --days is a subcommand-local flag,
	// "7" is its value, "archive-prune" is a known subcommand.
	rootCmd := NewRootCommand()

	// when
	got := DefaultToScan(rootCmd, []string{"--days", "7", "archive-prune"})

	// then: "archive-prune" reordered to front
	if len(got) != 3 || got[0] != "archive-prune" {
		t.Errorf("expected [archive-prune --days 7], got %v", got)
	}
}

func TestDefaultToScan_MultipleUnknownPositionalsNoSubcommand(t *testing.T) {
	// given: multiple unknown positional args with no subcommand
	rootCmd := NewRootCommand()

	// when
	got := DefaultToScan(rootCmd, []string{"path1", "path2"})

	// then: no subcommand found — prepend scan
	if len(got) != 3 || got[0] != "scan" {
		t.Errorf("expected [scan path1 path2], got %v", got)
	}
}
