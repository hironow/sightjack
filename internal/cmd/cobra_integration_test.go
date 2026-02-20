package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"go.opentelemetry.io/otel/trace"
)

func TestCobraRouting_Init(t *testing.T) {
	// given: a fresh temp directory with stdin providing init answers
	dir := t.TempDir()
	input := "TestTeam\nTestProject\n\n\n"

	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"init", dir})
	rootCmd.SetIn(strings.NewReader(input))
	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})

	// when
	err := rootCmd.Execute()

	// then
	if err != nil {
		t.Fatalf("init command failed: %v", err)
	}
	cfgFile := filepath.Join(dir, ".siren", "config.yaml")
	if _, statErr := os.Stat(cfgFile); os.IsNotExist(statErr) {
		t.Errorf("expected config file at %s", cfgFile)
	}
}

func TestCobraRouting_Doctor(t *testing.T) {
	// given: a temp directory (no config — doctor prints to os.Stdout directly)
	dir := t.TempDir()

	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"doctor", dir})
	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})

	// when: doctor runs to completion (may error since config is missing)
	err := rootCmd.Execute()

	// then: doctor should return an error about failed checks (no config)
	// but should not panic or return an unexpected error
	if err != nil && !strings.Contains(err.Error(), "check(s) failed") {
		t.Fatalf("unexpected error from doctor: %v", err)
	}
}

func TestCobraRouting_Show_NoState(t *testing.T) {
	// given: a directory with no .siren/state.json
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer

	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"show", dir})
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)

	// when
	err := rootCmd.Execute()

	// then: should return error about missing scan
	if err == nil {
		t.Fatal("expected error when no state exists")
	}
}

func TestCobraRouting_ArchivePrune_EmptyDir(t *testing.T) {
	// given: an empty temp directory
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer

	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"archive-prune", dir})
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)

	// when
	err := rootCmd.Execute()

	// then: should succeed (no expired files to prune)
	if err != nil {
		t.Fatalf("archive-prune should succeed on empty dir: %v", err)
	}
}

func TestCobraRouting_Version(t *testing.T) {
	// given
	var stdout bytes.Buffer

	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"--version"})
	rootCmd.SetOut(&stdout)

	// when
	err := rootCmd.Execute()

	// then
	if err != nil {
		t.Fatalf("--version failed: %v", err)
	}
	if !strings.Contains(stdout.String(), version) {
		t.Errorf("expected version %q in output, got: %s", version, stdout.String())
	}
}

func TestCobraRouting_UnknownCommand(t *testing.T) {
	// given
	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"nonexistent-command"})
	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})

	// when
	err := rootCmd.Execute()

	// then
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
}

func TestCobraFlagInheritance_Verbose(t *testing.T) {
	// given: pass --verbose to a subcommand
	dir := t.TempDir()
	var stdout bytes.Buffer

	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"--verbose", "doctor", dir})
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&bytes.Buffer{})

	// when: should not error on flag parsing
	_ = rootCmd.Execute()

	// then: verbose flag should be inherited
	if !verbose {
		t.Error("expected verbose flag to be set via persistent flag inheritance")
	}
}

func TestCobraFlagInheritance_Config(t *testing.T) {
	// given: pass --config to a subcommand
	dir := t.TempDir()

	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"--config", "/custom/path.yaml", "show", dir})
	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})

	// when
	_ = rootCmd.Execute()

	// then: config flag should be inherited and set
	if cfgPath != "/custom/path.yaml" {
		t.Errorf("expected cfgPath to be /custom/path.yaml, got %q", cfgPath)
	}
}

func TestPersistentHooks_SpanContext(t *testing.T) {
	// given: execute a command that triggers PersistentPreRunE
	dir := t.TempDir()
	var spanFromContext trace.Span

	rootCmd := NewRootCommand()

	// Add a test-only subcommand that captures the span from context.
	testCmd := rootCmd
	for _, sub := range rootCmd.Commands() {
		if sub.Name() == "doctor" {
			originalRunE := sub.RunE
			sub.RunE = func(cmd *cobra.Command, args []string) error {
				spanFromContext = trace.SpanFromContext(cmd.Context())
				return originalRunE(cmd, args)
			}
			break
		}
	}
	_ = testCmd

	rootCmd.SetArgs([]string{"doctor", dir})
	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})

	// when
	_ = rootCmd.Execute()

	// then: PersistentPreRunE should have set a span in the context
	if spanFromContext == nil {
		t.Fatal("expected span to be set in command context by PersistentPreRunE")
	}
}
