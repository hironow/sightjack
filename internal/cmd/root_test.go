package cmd

// white-box-reason: cobra command construction: NewRootCommand and CLI routing are unexported

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/hironow/sightjack/internal/domain"
)


// newTestCommand creates a cobra.Command with --config flag registered on Flags()
// for direct unit testing of resolveConfigPath (without cobra's execution-time
// persistent flag merging).
func newTestCommand() *cobra.Command {
	c := &cobra.Command{}
	c.Flags().StringVarP(&cfgPath, "config", "c", ".siren/config.yaml", "Config file path")
	return c
}

func TestResolveConfigPath_RelativeWithBaseDir(t *testing.T) {
	// given: command with --config explicitly set to a relative path
	c := newTestCommand()
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
	c := newTestCommand()
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
	c := newTestCommand()

	// when
	got := resolveConfigPath(c, "/repo")

	// then: should use ConfigPath(baseDir)
	want := domain.ConfigPath("/repo")
	if got != want {
		t.Errorf("resolveConfigPath: expected %q, got %q", want, got)
	}
}

func TestTTYDevices_Unix(t *testing.T) {
	// given — Unix GOOS values
	for _, goos := range []string{"darwin", "linux", "freebsd"} {
		// when
		devices := ttyDevices(goos)

		// then — /dev/tty should be first
		if len(devices) < 1 {
			t.Fatalf("ttyDevices(%q) returned empty", goos)
		}
		if devices[0] != "/dev/tty" {
			t.Errorf("ttyDevices(%q)[0] = %q, want /dev/tty", goos, devices[0])
		}
	}
}

func TestTTYDevices_Windows(t *testing.T) {
	// given — Windows GOOS
	// when
	devices := ttyDevices("windows")

	// then — CONIN$ should be first
	if len(devices) < 1 {
		t.Fatal("ttyDevices(windows) returned empty")
	}
	if devices[0] != "CONIN$" {
		t.Errorf("ttyDevices(windows)[0] = %q, want CONIN$", devices[0])
	}
}

func TestNoColorFlag_Exists(t *testing.T) {
	// given
	rootCmd := NewRootCommand()

	// when
	f := rootCmd.PersistentFlags().Lookup("no-color")

	// then
	if f == nil {
		t.Fatal("--no-color PersistentFlag not found")
	}
	if f.DefValue != "false" {
		t.Errorf("--no-color default = %q, want %q", f.DefValue, "false")
	}
}

func TestRootCmd_OutputFlagExists(t *testing.T) {
	// given
	rootCmd := NewRootCommand()

	// when
	f := rootCmd.PersistentFlags().Lookup("output")

	// then
	if f == nil {
		t.Fatal("--output flag not found")
	}
	if f.DefValue != "text" {
		t.Errorf("default = %q, want text", f.DefValue)
	}
	if f.Shorthand != "o" {
		t.Errorf("shorthand = %q, want o", f.Shorthand)
	}
}

func TestRootCmd_VerboseFlagExists(t *testing.T) {
	// given
	rootCmd := NewRootCommand()

	// when
	f := rootCmd.PersistentFlags().Lookup("verbose")

	// then
	if f == nil {
		t.Fatal("--verbose flag not found")
	}
	if f.DefValue != "false" {
		t.Errorf("default = %q, want false", f.DefValue)
	}
	if f.Shorthand != "v" {
		t.Errorf("shorthand = %q, want v", f.Shorthand)
	}
}

func TestRootCmd_VerboseIncreasesStderrOutput(t *testing.T) {
	// given: initialized project
	dir := t.TempDir()
	sirenDir := filepath.Join(dir, ".siren")
	os.MkdirAll(sirenDir, 0o755)
	os.WriteFile(domain.ConfigPath(dir), []byte("tracker:\n  team: TEST\n  project: TEST\nlang: en\n"), 0o644)

	// when: run status without verbose
	root1 := NewRootCommand()
	var stdout1, stderr1 bytes.Buffer
	root1.SetOut(&stdout1)
	root1.SetErr(&stderr1)
	root1.SetArgs([]string{"status", dir})
	root1.Execute()

	// when: run status WITH verbose
	root2 := NewRootCommand()
	var stdout2, stderr2 bytes.Buffer
	root2.SetOut(&stdout2)
	root2.SetErr(&stderr2)
	root2.SetArgs([]string{"-v", "status", dir})
	root2.Execute()

	// then: verbose should produce at least as much stderr
	if stderr2.Len() < stderr1.Len() {
		t.Errorf("verbose stderr (%d bytes) should be >= non-verbose stderr (%d bytes)",
			stderr2.Len(), stderr1.Len())
	}
}

func TestRootCmd_NoColorSetsEnv(t *testing.T) {
	// given
	origVal := os.Getenv("NO_COLOR")
	os.Unsetenv("NO_COLOR")
	t.Cleanup(func() {
		if origVal != "" {
			os.Setenv("NO_COLOR", origVal)
		} else {
			os.Unsetenv("NO_COLOR")
		}
	})

	dir := t.TempDir()
	sirenDir := filepath.Join(dir, ".siren")
	os.MkdirAll(sirenDir, 0o755)
	os.WriteFile(domain.ConfigPath(dir), []byte("tracker:\n  team: TEST\n  project: TEST\nlang: en\n"), 0o644)

	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"--no-color", "status", dir})

	// when
	root.Execute()

	// then
	if got := os.Getenv("NO_COLOR"); got == "" {
		t.Error("expected NO_COLOR env to be set after --no-color flag")
	}
	output := buf.String()
	if strings.Contains(output, "\033[") {
		t.Errorf("--no-color output should not contain ANSI codes, got: %q", output)
	}
}
