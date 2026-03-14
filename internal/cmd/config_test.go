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

func TestConfigCmd_SetAllKeys(t *testing.T) {
	tests := []struct {
		key      string
		value    string
		contains string
	}{
		// tracker keys
		{"tracker.team", "NEWTEAM", "team: NEWTEAM"},
		{"tracker.project", "MyProj", "project: MyProj"},
		{"tracker.cycle", "2026-Q1", "cycle: 2026-Q1"},
		// lang
		{"lang", "en", "lang: en"},
		// strictness
		{"strictness.default", "alert", "default: alert"},
		// scan
		{"scan.chunk_size", "20", "chunk_size: 20"},
		{"scan.max_concurrency", "4", "max_concurrency: 4"},
		// assistant aliases
		{"model", "sonnet", "model: sonnet"},
		{"claude_cmd", "claude-dev", "claude_cmd: claude-dev"},
		{"timeout_sec", "120", "timeout_sec: 120"},
		// scribe
		{"scribe.enabled", "true", "enabled: true"},
		{"scribe.auto_discuss_rounds", "3", "auto_discuss_rounds: 3"},
		// retry
		{"retry.max_attempts", "5", "max_attempts: 5"},
		{"retry.base_delay_sec", "10", "base_delay_sec: 10"},
		// gate
		{"gate.auto_approve", "true", "auto_approve: true"},
		{"gate.notify_cmd", "notify-send", "notify_cmd: notify-send"},
		{"gate.approve_cmd", "approve-it", "approve_cmd: approve-it"},
		{"gate.review_cmd", "pnpm lint", "review_cmd: pnpm lint"},
		{"gate.review_budget", "3", "review_budget: 3"},
		{"gate.wait_timeout", "45m0s", "wait_timeout: 45m0s"},
		// labels
		{"labels.enabled", "true", "enabled: true"},
		{"labels.prefix", "sj-", "prefix: sj-"},
		{"labels.ready_label", "ready-to-wave", "ready_label: ready-to-wave"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			// given
			dir := initProject(t)

			rootCmd := cmd.NewRootCommand()
			buf := new(bytes.Buffer)
			rootCmd.SetOut(buf)
			rootCmd.SetErr(buf)
			rootCmd.SetArgs([]string{"config", "set", tt.key, tt.value, dir})

			// when
			err := rootCmd.Execute()

			// then
			if err != nil {
				t.Fatalf("config set %s=%s failed: %v", tt.key, tt.value, err)
			}
			data, readErr := os.ReadFile(domain.ConfigPath(dir))
			if readErr != nil {
				t.Fatalf("read config: %v", readErr)
			}
			if !strings.Contains(string(data), tt.contains) {
				t.Errorf("expected %q in config, got:\n%s", tt.contains, string(data))
			}
		})
	}
}

func TestConfigCmd_Set_RejectsUnknownKey(t *testing.T) {
	// given
	dir := initProject(t)
	rootCmd := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"config", "set", "nonexistent.key", "value", dir})

	// when
	err := rootCmd.Execute()

	// then
	if err == nil {
		t.Fatal("expected error for unknown config key")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Errorf("expected 'unknown' in error, got: %s", err.Error())
	}
}

func TestConfigCmd_Set_RejectsInvalidLang(t *testing.T) {
	// given
	dir := initProject(t)
	rootCmd := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"config", "set", "lang", "fr", dir})

	// when
	err := rootCmd.Execute()

	// then
	if err == nil {
		t.Fatal("expected error for invalid lang 'fr'")
	}
}

func TestConfigCmd_Set_RejectsInvalidIntValue(t *testing.T) {
	// given
	dir := initProject(t)
	rootCmd := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"config", "set", "scan.chunk_size", "not-a-number", dir})

	// when
	err := rootCmd.Execute()

	// then
	if err == nil {
		t.Fatal("expected error for non-integer value")
	}
}

func TestConfigCmd_Set_RejectsInvalidBoolValue(t *testing.T) {
	// given
	dir := initProject(t)
	rootCmd := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"config", "set", "gate.auto_approve", "maybe", dir})

	// when
	err := rootCmd.Execute()

	// then
	if err == nil {
		t.Fatal("expected error for invalid bool value")
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
