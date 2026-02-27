package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunCmd_FailsWithoutInit(t *testing.T) {
	// given: empty directory with no .siren/ or config
	dir := t.TempDir()

	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--config", dir + "/siren.yaml", "run", dir})

	// when
	err := cmd.Execute()

	// then: should fail with init guidance
	if err == nil {
		t.Fatal("expected error for uninitialized state, got nil")
	}
	got := err.Error()
	if !strings.Contains(got, "init") {
		t.Errorf("expected error to mention 'init', got: %s", got)
	}
}
