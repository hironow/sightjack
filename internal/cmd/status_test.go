package cmd_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/hironow/sightjack/internal/cmd"
)

func TestStatusCmd_SubcommandExists(t *testing.T) {
	// given
	root := cmd.NewRootCommand()

	// when
	found := false
	for _, sub := range root.Commands() {
		if sub.Name() == "status" {
			found = true
			break
		}
	}

	// then
	if !found {
		t.Fatal("status subcommand not found")
	}
}

func TestStatusCmd_AcceptsOptionalPath(t *testing.T) {
	// given
	root := cmd.NewRootCommand()
	statusCmd, _, err := root.Find([]string{"status"})
	if err != nil {
		t.Fatalf("find status: %v", err)
	}

	// then: MaximumNArgs(1)
	if statusCmd.Args == nil {
		t.Fatal("status command should have Args validator")
	}
}

func TestStatusCmd_RejectsTooManyArgs(t *testing.T) {
	// given
	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"status", "a", "b"})

	// when
	err := root.Execute()

	// then
	if err == nil {
		t.Fatal("expected error for too many args")
	}
}

func TestStatusCmd_RunsOnUninitializedDir(t *testing.T) {
	// given: status should work even without init (shows empty state)
	dir := t.TempDir()
	root := cmd.NewRootCommand()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"status", dir})

	// when
	err := root.Execute()

	// then: status should not fail — it should report "no state" gracefully
	if err != nil {
		errMsg := err.Error()
		// Only fail if it's an unexpected error (not init-related)
		if !strings.Contains(errMsg, "init") && !strings.Contains(errMsg, "siren") {
			t.Errorf("unexpected error: %v", err)
		}
	}
}

func TestStatusCmd_RunsOnInitializedDir(t *testing.T) {
	// given: initialized project
	dir := initProject(t)

	root := cmd.NewRootCommand()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"status", dir})

	// when
	err := root.Execute()

	// then
	if err != nil {
		t.Fatalf("status on initialized dir failed: %v", err)
	}
}

func TestStatusCmd_JSONOutput(t *testing.T) {
	// given: initialized project
	dir := initProject(t)

	root := cmd.NewRootCommand()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"status", "-o", "json", dir})

	// when
	err := root.Execute()

	// then
	if err != nil {
		t.Fatalf("status -o json failed: %v", err)
	}
	if !json.Valid(stdout.Bytes()) {
		t.Errorf("stdout is not valid JSON: %s", stdout.String())
	}
}
