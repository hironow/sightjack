package cmd
// white-box-reason: cobra command construction: NewRootCommand and CLI routing are unexported

import (
	"testing"
)

func TestNeedsDefaultScan_NoArgs(t *testing.T) {
	// given
	rootCmd := NewRootCommand()

	// when
	got := NeedsDefaultScan(rootCmd, []string{})

	// then
	if !got {
		t.Error("expected true for empty args")
	}
}

func TestNeedsDefaultScan_NilArgs(t *testing.T) {
	// given
	rootCmd := NewRootCommand()

	// when
	got := NeedsDefaultScan(rootCmd, nil)

	// then
	if !got {
		t.Error("expected true for nil args")
	}
}

func TestNeedsDefaultScan_PathOnly(t *testing.T) {
	// given: just a path — no subcommand found → needs default
	rootCmd := NewRootCommand()

	// when
	got := NeedsDefaultScan(rootCmd, []string{"."})

	// then
	if !got {
		t.Error("expected true for path-only args")
	}
}

func TestNeedsDefaultScan_ScanLocalFlagOnly(t *testing.T) {
	// given: only flags, no subcommand
	rootCmd := NewRootCommand()

	// when
	got := NeedsDefaultScan(rootCmd, []string{"--json"})

	// then
	if !got {
		t.Error("expected true for flag-only args")
	}
}

func TestNeedsDefaultScan_ScanLocalFlagWithPath(t *testing.T) {
	// given: --json flag (scan-local) with path
	rootCmd := NewRootCommand()

	// when
	got := NeedsDefaultScan(rootCmd, []string{"--json", "."})

	// then
	if !got {
		t.Error("expected true for scan-local flag with path")
	}
}

func TestNeedsDefaultScan_ExplicitSubcommand(t *testing.T) {
	// given: explicit subcommand
	rootCmd := NewRootCommand()

	// when
	got := NeedsDefaultScan(rootCmd, []string{"version", "--json"})

	// then
	if got {
		t.Error("expected false for explicit subcommand")
	}
}

func TestNeedsDefaultScan_ExplicitScan(t *testing.T) {
	// given: explicit scan subcommand
	rootCmd := NewRootCommand()

	// when
	got := NeedsDefaultScan(rootCmd, []string{"scan", "."})

	// then
	if got {
		t.Error("expected false for explicit scan")
	}
}

func TestNeedsDefaultScan_VersionFlag(t *testing.T) {
	// given: --version flag
	rootCmd := NewRootCommand()

	// when
	got := NeedsDefaultScan(rootCmd, []string{"--version"})

	// then
	if got {
		t.Error("expected false for --version")
	}
}

func TestNeedsDefaultScan_HelpFlag(t *testing.T) {
	// given: --help flag
	rootCmd := NewRootCommand()

	// when
	got := NeedsDefaultScan(rootCmd, []string{"--help"})

	// then
	if got {
		t.Error("expected false for --help")
	}
}

func TestNeedsDefaultScan_HelpSubcommand(t *testing.T) {
	// given: help subcommand (cobra-injected)
	rootCmd := NewRootCommand()

	// when
	got := NeedsDefaultScan(rootCmd, []string{"help"})

	// then
	if got {
		t.Error("expected false for help subcommand")
	}
}

func TestNeedsDefaultScan_CompletionSubcommand(t *testing.T) {
	// given: completion subcommand (cobra-injected)
	rootCmd := NewRootCommand()

	// when
	got := NeedsDefaultScan(rootCmd, []string{"completion", "bash"})

	// then
	if got {
		t.Error("expected false for completion subcommand")
	}
}

func TestNeedsDefaultScan_PersistentFlagBeforeSubcommand(t *testing.T) {
	// given: --verbose before doctor subcommand
	rootCmd := NewRootCommand()

	// when
	got := NeedsDefaultScan(rootCmd, []string{"--verbose", "doctor", "."})

	// then
	if got {
		t.Error("expected false when subcommand is present after flags")
	}
}

func TestNeedsDefaultScan_ValueFlagBeforeSubcommand(t *testing.T) {
	// given: --config takes a value, waves is the subcommand
	rootCmd := NewRootCommand()

	// when
	got := NeedsDefaultScan(rootCmd, []string{"--config", "custom.yaml", "waves"})

	// then
	if got {
		t.Error("expected false when subcommand follows value flag")
	}
}

func TestNeedsDefaultScan_ValueFlagBeforePath(t *testing.T) {
	// given: --lang ja /path — /path is not a subcommand
	rootCmd := NewRootCommand()

	// when
	got := NeedsDefaultScan(rootCmd, []string{"--lang", "ja", "/some/path"})

	// then
	if !got {
		t.Error("expected true when only path follows value flag")
	}
}

func TestNeedsDefaultScan_MultipleUnknownPositionals(t *testing.T) {
	// given: multiple unknown positional args with no subcommand
	rootCmd := NewRootCommand()

	// when
	got := NeedsDefaultScan(rootCmd, []string{"path1", "path2"})

	// then
	if !got {
		t.Error("expected true for unknown positionals")
	}
}

func TestReorderArgs_SubcommandAtFront(t *testing.T) {
	// given: subcommand already at front
	rootCmd := NewRootCommand()

	// when
	got := ReorderArgs(rootCmd, []string{"version", "--json"})

	// then
	if len(got) != 2 || got[0] != "version" || got[1] != "--json" {
		t.Errorf("expected [version --json] unchanged, got %v", got)
	}
}

func TestReorderArgs_FlagBeforeSubcommand(t *testing.T) {
	// given: --verbose before doctor
	rootCmd := NewRootCommand()

	// when
	got := ReorderArgs(rootCmd, []string{"--verbose", "doctor", "."})

	// then
	if len(got) != 3 || got[0] != "doctor" {
		t.Errorf("expected [doctor --verbose .], got %v", got)
	}
}

func TestReorderArgs_ValueFlagBeforeSubcommand(t *testing.T) {
	// given: --config custom.yaml waves
	rootCmd := NewRootCommand()

	// when
	got := ReorderArgs(rootCmd, []string{"--config", "custom.yaml", "waves"})

	// then
	if len(got) != 3 || got[0] != "waves" {
		t.Errorf("expected [waves --config custom.yaml], got %v", got)
	}
}

func TestReorderArgs_ShortValueFlagBeforeSubcommand(t *testing.T) {
	// given: -c custom.yaml doctor
	rootCmd := NewRootCommand()

	// when
	got := ReorderArgs(rootCmd, []string{"-c", "custom.yaml", "doctor"})

	// then
	if len(got) != 3 || got[0] != "doctor" {
		t.Errorf("expected [doctor -c custom.yaml], got %v", got)
	}
}

func TestReorderArgs_ValueFlagEqualsForm(t *testing.T) {
	// given: --config=custom.yaml waves
	rootCmd := NewRootCommand()

	// when
	got := ReorderArgs(rootCmd, []string{"--config=custom.yaml", "waves"})

	// then
	if len(got) != 2 || got[0] != "waves" {
		t.Errorf("expected [waves --config=custom.yaml], got %v", got)
	}
}

func TestReorderArgs_ScanLocalFlagBeforeScan(t *testing.T) {
	// given: --json scan — scan-local flag before subcommand
	rootCmd := NewRootCommand()

	// when
	got := ReorderArgs(rootCmd, []string{"--json", "scan"})

	// then
	if len(got) != 2 || got[0] != "scan" || got[1] != "--json" {
		t.Errorf("expected [scan --json], got %v", got)
	}
}

func TestReorderArgs_UnknownFlagBeforeSubcommand(t *testing.T) {
	// given: --days 7 archive-prune
	rootCmd := NewRootCommand()

	// when
	got := ReorderArgs(rootCmd, []string{"--days", "7", "archive-prune"})

	// then
	if len(got) != 3 || got[0] != "archive-prune" {
		t.Errorf("expected [archive-prune --days 7], got %v", got)
	}
}

func TestReorderArgs_NoSubcommand(t *testing.T) {
	// given: no subcommand present
	rootCmd := NewRootCommand()

	// when
	got := ReorderArgs(rootCmd, []string{"--json", "."})

	// then: should return unchanged
	if len(got) != 2 || got[0] != "--json" || got[1] != "." {
		t.Errorf("expected [--json .] unchanged, got %v", got)
	}
}

func TestReorderArgs_Empty(t *testing.T) {
	// given
	rootCmd := NewRootCommand()

	// when
	got := ReorderArgs(rootCmd, []string{})

	// then
	if len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

func TestReorderArgs_HelpSubcommandWithTopic(t *testing.T) {
	// given: help scan
	rootCmd := NewRootCommand()

	// when
	got := ReorderArgs(rootCmd, []string{"help", "scan"})

	// then: help at front, no reordering needed
	if len(got) != 2 || got[0] != "help" || got[1] != "scan" {
		t.Errorf("expected [help scan] unchanged, got %v", got)
	}
}

func TestReorderArgs_BoolFlagExplicitValueBeforeSubcommand(t *testing.T) {
	// given: --dry-run false waves
	rootCmd := NewRootCommand()

	// when
	got := ReorderArgs(rootCmd, []string{"--dry-run", "false", "waves"})

	// then: "waves" reordered to front
	if len(got) != 3 || got[0] != "waves" {
		t.Errorf("expected [waves --dry-run false], got %v", got)
	}
}
