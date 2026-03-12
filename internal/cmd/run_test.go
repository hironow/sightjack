package cmd_test

import (
	"bytes"
	"strings"
	"testing"

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
