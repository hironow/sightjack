package cmd

import (
	"strings"
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
	// given: --verbose (persistent) before explicit subcommand
	rootCmd := NewRootCommand()

	// when
	got := DefaultToScan(rootCmd, []string{"--verbose", "doctor", "."})

	// then: "doctor" is a known subcommand, should not change
	if len(got) != 3 || !strings.Contains(strings.Join(got, " "), "doctor") {
		t.Errorf("expected unchanged [--verbose doctor .], got %v", got)
	}
}
