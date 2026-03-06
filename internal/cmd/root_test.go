package cmd

// white-box-reason: cobra command construction: NewRootCommand and CLI routing are unexported

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/hironow/sightjack/internal/domain"
)

func TestResolveBaseDir_NonExistentPath(t *testing.T) {
	// given
	nonExistent := filepath.Join(t.TempDir(), "does-not-exist")

	// when
	_, err := resolveBaseDir([]string{nonExistent})

	// then
	if err == nil {
		t.Fatal("expected error for non-existent path")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestResolveBaseDir_FileNotDir(t *testing.T) {
	// given
	f := filepath.Join(t.TempDir(), "somefile.txt")
	os.WriteFile(f, []byte("x"), 0644)

	// when
	_, err := resolveBaseDir([]string{f})

	// then
	if err == nil {
		t.Fatal("expected error for file (not directory)")
	}
	if !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("expected 'not a directory' in error, got: %v", err)
	}
}

func TestResolveBaseDir_ValidDir(t *testing.T) {
	// given
	dir := t.TempDir()

	// when
	got, err := resolveBaseDir([]string{dir})

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	abs, _ := filepath.Abs(dir)
	if got != abs {
		t.Errorf("got %q, want %q", got, abs)
	}
}

func TestResolveBaseDir_NoArgs_UsesCwd(t *testing.T) {
	// when
	got, err := resolveBaseDir(nil)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cwd, _ := os.Getwd()
	if got != cwd {
		t.Errorf("got %q, want cwd %q", got, cwd)
	}
}

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
