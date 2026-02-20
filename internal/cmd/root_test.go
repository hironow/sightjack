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
