package cmd_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/hironow/sightjack/internal/cmd"
)

func TestRunCmd_FailsWithoutInit(t *testing.T) {
	// given: empty directory with no .siren/ or config
	dir := t.TempDir()

	rootCmd := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"--config", dir + "/siren.yaml", "run", dir})

	// when
	err := rootCmd.Execute()

	// then: should fail with init guidance
	if err == nil {
		t.Fatal("expected error for uninitialized state, got nil")
	}
	got := err.Error()
	if !strings.Contains(got, "init") {
		t.Errorf("expected error to mention 'init', got: %s", got)
	}
}

func TestRunCmd_BootstrapWithInit(t *testing.T) {
	// given: initialized repo with .siren/ structure + minimal config
	dir := t.TempDir()
	sirenDir := filepath.Join(dir, ".siren")
	for _, sub := range []string{".run", "events", "inbox", "outbox", "archive"} {
		os.MkdirAll(filepath.Join(sirenDir, sub), 0755)
	}
	// Minimal config (empty YAML uses defaults)
	os.WriteFile(filepath.Join(sirenDir, "config.yaml"), []byte("claude_cmd: echo\n"), 0644)

	// git init (required by preflight check)
	gitInit := exec.Command("git", "init")
	gitInit.Dir = dir
	if out, err := gitInit.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}

	// when: run with --dry-run --session-mode=new --idle-timeout=-1s
	rootCmd := cmd.NewRootCommand()
	rootCmd.SetArgs([]string{"--config", filepath.Join(sirenDir, "config.yaml"), "run", "--dry-run", "--session-mode=new", "--idle-timeout=-1s", dir})
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	err := rootCmd.Execute()

	// then: should NOT fail with "init" or "config not found"
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "init") || strings.Contains(errMsg, "config not found") {
			t.Fatalf("bootstrap failed: %v", err)
		}
		// Other errors are acceptable — bootstrap passed
		t.Logf("run completed with non-init error (bootstrap OK): %v", err)
	}
}

func TestRunCmd_FlagsExist(t *testing.T) {
	// given
	rootCmd := cmd.NewRootCommand()
	var runCmd *cobra.Command
	for _, sub := range rootCmd.Commands() {
		if sub.Name() == "run" {
			runCmd = sub
			break
		}
	}
	if runCmd == nil {
		t.Fatal("run subcommand not found")
	}

	// when/then
	flags := []struct {
		name     string
		defValue string
	}{
		{"idle-timeout", "30m0s"},
		{"auto-approve", "false"},
		{"approve-cmd", ""},
		{"notify-cmd", ""},
		{"review-cmd", ""},
		{"session-mode", ""},
		{"strictness", ""},
	}
	for _, tc := range flags {
		f := runCmd.Flags().Lookup(tc.name)
		if f == nil {
			t.Errorf("--%s flag not found", tc.name)
			continue
		}
		if f.DefValue != tc.defValue {
			t.Errorf("--%s default = %q, want %q", tc.name, f.DefValue, tc.defValue)
		}
	}
}

func TestRunCmd_StrictnessShortAlias(t *testing.T) {
	// given
	rootCmd := cmd.NewRootCommand()
	var runCmd *cobra.Command
	for _, sub := range rootCmd.Commands() {
		if sub.Name() == "run" {
			runCmd = sub
			break
		}
	}
	if runCmd == nil {
		t.Fatal("run subcommand not found")
	}

	// when
	f := runCmd.Flags().ShorthandLookup("s")

	// then
	if f == nil {
		t.Fatal("-s shorthand not found on run command")
	}
	if f.Name != "strictness" {
		t.Errorf("-s resolves to %q, want %q", f.Name, "strictness")
	}
}
