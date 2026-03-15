package cmd_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/hironow/sightjack/internal/cmd"
)

// Pipeline commands: scan, waves, select, discuss, apply, show, nextgen, adr
// These all read from stdin and/or accept [path]. Unit tests verify:
// - Subcommand exists
// - Argument validation
// - Empty stdin handling (where applicable)

func TestScanCmd_SubcommandExists(t *testing.T) {
	root := cmd.NewRootCommand()
	found := false
	for _, sub := range root.Commands() {
		if sub.Name() == "scan" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("scan subcommand not found")
	}
}

func TestScanCmd_RejectsTooManyArgs(t *testing.T) {
	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"scan", "a", "b"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for too many args")
	}
}

func TestScanCmd_JSONFlagExists(t *testing.T) {
	root := cmd.NewRootCommand()
	scanCmd, _, err := root.Find([]string{"scan"})
	if err != nil {
		t.Fatalf("find scan: %v", err)
	}

	f := scanCmd.Flags().Lookup("json")
	if f == nil {
		t.Fatal("--json flag not found on scan")
	}
	if f.Shorthand != "j" {
		t.Errorf("--json shorthand = %q, want %q", f.Shorthand, "j")
	}
}

func TestScanCmd_StrictnessFlagExists(t *testing.T) {
	root := cmd.NewRootCommand()
	scanCmd, _, err := root.Find([]string{"scan"})
	if err != nil {
		t.Fatalf("find scan: %v", err)
	}

	f := scanCmd.Flags().Lookup("strictness")
	if f == nil {
		t.Fatal("--strictness flag not found on scan")
	}
}

func TestWavesCmd_SubcommandExists(t *testing.T) {
	root := cmd.NewRootCommand()
	found := false
	for _, sub := range root.Commands() {
		if sub.Name() == "waves" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("waves subcommand not found")
	}
}

func TestWavesCmd_RejectsTooManyArgs(t *testing.T) {
	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"waves", "a", "b"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for too many args")
	}
}

func TestWavesCmd_FailsOnEmptyStdin(t *testing.T) {
	// waves requires config, so use an initialized project
	dir := initProject(t)
	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetIn(strings.NewReader(""))
	root.SetArgs([]string{"waves", dir})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for empty stdin")
	}
	if !strings.Contains(err.Error(), "stdin") && !strings.Contains(err.Error(), "empty") {
		t.Errorf("expected error about stdin/empty input, got: %s", err.Error())
	}
}

func TestSelectCmd_SubcommandExists(t *testing.T) {
	root := cmd.NewRootCommand()
	found := false
	for _, sub := range root.Commands() {
		if sub.Name() == "select" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("select subcommand not found")
	}
}

func TestSelectCmd_NoArgsAllowed(t *testing.T) {
	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"select", "extra"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error — select takes no positional args")
	}
}

func TestSelectCmd_FailsOnEmptyStdin(t *testing.T) {
	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetIn(strings.NewReader(""))
	root.SetArgs([]string{"select"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for empty stdin")
	}
	if !strings.Contains(err.Error(), "stdin") {
		t.Errorf("expected error to mention 'stdin', got: %s", err.Error())
	}
}

func TestDiscussCmd_SubcommandExists(t *testing.T) {
	root := cmd.NewRootCommand()
	found := false
	for _, sub := range root.Commands() {
		if sub.Name() == "discuss" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("discuss subcommand not found")
	}
}

func TestDiscussCmd_RejectsTooManyArgs(t *testing.T) {
	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"discuss", "a", "b"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for too many args")
	}
}

func TestApplyCmd_SubcommandExists(t *testing.T) {
	root := cmd.NewRootCommand()
	found := false
	for _, sub := range root.Commands() {
		if sub.Name() == "apply" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("apply subcommand not found")
	}
}

func TestApplyCmd_RejectsTooManyArgs(t *testing.T) {
	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"apply", "a", "b"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for too many args")
	}
}

func TestShowCmd_SubcommandExists(t *testing.T) {
	root := cmd.NewRootCommand()
	found := false
	for _, sub := range root.Commands() {
		if sub.Name() == "show" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("show subcommand not found")
	}
}

func TestShowCmd_RejectsTooManyArgs(t *testing.T) {
	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"show", "a", "b"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for too many args")
	}
}

func TestNextgenCmd_SubcommandExists(t *testing.T) {
	root := cmd.NewRootCommand()
	found := false
	for _, sub := range root.Commands() {
		if sub.Name() == "nextgen" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("nextgen subcommand not found")
	}
}

func TestNextgenCmd_RejectsTooManyArgs(t *testing.T) {
	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"nextgen", "a", "b"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for too many args")
	}
}

func TestADRCmd_SubcommandExists(t *testing.T) {
	root := cmd.NewRootCommand()
	found := false
	for _, sub := range root.Commands() {
		if sub.Name() == "adr" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("adr subcommand not found")
	}
}

func TestADRCmd_RejectsTooManyArgs(t *testing.T) {
	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"adr", "a", "b"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for too many args")
	}
}

func TestADRCmd_FailsOnEmptyStdin(t *testing.T) {
	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetIn(strings.NewReader(""))
	root.SetArgs([]string{"adr"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for empty stdin")
	}
	if !strings.Contains(err.Error(), "stdin") {
		t.Errorf("expected error to mention 'stdin', got: %s", err.Error())
	}
}

func TestSelectCmd_FailsOnInvalidJSON(t *testing.T) {
	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetIn(strings.NewReader("not valid json"))
	root.SetArgs([]string{"select"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for invalid JSON on stdin")
	}
}

func TestADRCmd_FailsOnInvalidJSON(t *testing.T) {
	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetIn(strings.NewReader("not valid json"))
	root.SetArgs([]string{"adr"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for invalid JSON on stdin")
	}
}
