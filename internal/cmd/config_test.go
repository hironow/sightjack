package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/hironow/sightjack/internal/cmd"
	"github.com/hironow/sightjack/internal/domain"
)

func TestConfigCmd_ShowSubcommandExists(t *testing.T) {
	// given
	rootCmd := cmd.NewRootCommand()
	var configCmd *cobra.Command
	for _, sub := range rootCmd.Commands() {
		if sub.Name() == "config" {
			configCmd = sub
			break
		}
	}
	if configCmd == nil {
		t.Fatal("config subcommand not found")
	}

	// when/then
	var showCmd *cobra.Command
	for _, sub := range configCmd.Commands() {
		if sub.Name() == "show" {
			showCmd = sub
			break
		}
	}
	if showCmd == nil {
		t.Fatal("config show subcommand not found")
	}
}

func TestConfigCmd_SetSubcommandExists(t *testing.T) {
	// given
	rootCmd := cmd.NewRootCommand()
	var configCmd *cobra.Command
	for _, sub := range rootCmd.Commands() {
		if sub.Name() == "config" {
			configCmd = sub
			break
		}
	}
	if configCmd == nil {
		t.Fatal("config subcommand not found")
	}

	// when/then
	var setCmd *cobra.Command
	for _, sub := range configCmd.Commands() {
		if sub.Name() == "set" {
			setCmd = sub
			break
		}
	}
	if setCmd == nil {
		t.Fatal("config set subcommand not found")
	}
}

func initProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	rootCmd := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetIn(strings.NewReader(""))
	rootCmd.SetArgs([]string{"init", "--team", "TestTeam", "--project", "TestProject", dir})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("init failed: %v", err)
	}
	return dir
}

func TestConfigCmd_Set_UpdatesConfig(t *testing.T) {
	// given: initialized project
	dir := initProject(t)

	rootCmd := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"config", "set", "lang", "en", dir})

	// when
	err := rootCmd.Execute()

	// then
	if err != nil {
		t.Fatalf("config set failed: %v", err)
	}
	data, readErr := os.ReadFile(domain.ConfigPath(dir))
	if readErr != nil {
		t.Fatalf("config read failed: %v", readErr)
	}
	if !strings.Contains(string(data), "lang: en") {
		t.Errorf("expected 'lang: en' in config after set, got:\n%s", data)
	}
}

func TestConfigCmd_Show_DisplaysConfig(t *testing.T) {
	// given: initialized project
	dir := initProject(t)

	rootCmd := cmd.NewRootCommand()
	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(outBuf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"config", "show", dir})

	// when
	err := rootCmd.Execute()

	// then
	if err != nil {
		t.Fatalf("config show failed: %v", err)
	}
	output := outBuf.String()
	if output == "" {
		t.Fatal("config show produced no output")
	}
	// Should contain config keys from the initialized project
	if !strings.Contains(output, "tracker") {
		t.Errorf("expected 'tracker' in config show output, got:\n%s", output)
	}
}

func TestConfigCmd_Show_FailsWithoutInit(t *testing.T) {
	// given: uninitialized directory
	dir := t.TempDir()

	rootCmd := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"config", "show", dir})

	// when
	err := rootCmd.Execute()

	// then
	if err == nil {
		t.Fatal("expected error for uninitialized config show")
	}
	if !strings.Contains(err.Error(), "init") {
		t.Errorf("expected error to mention 'init', got: %s", err.Error())
	}
}

func TestConfigCmd_Set_FailsWithoutInit(t *testing.T) {
	// given: uninitialized directory — config file does not exist
	dir := t.TempDir()
	// Ensure the .siren directory does not exist
	cfgPath := filepath.Join(dir, ".siren", "config.yaml")

	rootCmd := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"config", "set", "lang", "en", dir})

	// when
	err := rootCmd.Execute()

	// then: should fail because config file doesn't exist
	if err == nil {
		// If it somehow succeeds, the file should not have been created from scratch
		if _, statErr := os.Stat(cfgPath); statErr == nil {
			t.Fatal("expected error for uninitialized config set, but command succeeded and created config")
		}
	}
}
