package cmd

// white-box-reason: cobra command construction: NewRootCommand and CLI routing are unexported

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestVersionCmd_TextOutput(t *testing.T) {
	// given
	var stdout bytes.Buffer
	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"version"})
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&bytes.Buffer{})

	// when
	err := rootCmd.Execute()

	// then
	if err != nil {
		t.Fatalf("version command failed: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "sightjack v") {
		t.Errorf("expected 'sightjack v' in output, got: %s", out)
	}
	if !strings.Contains(out, "commit:") {
		t.Errorf("expected 'commit:' in output, got: %s", out)
	}
	if !strings.Contains(out, "go:") {
		t.Errorf("expected 'go:' in output, got: %s", out)
	}
}

func TestVersionCmd_JSONOutput(t *testing.T) {
	// given
	var stdout bytes.Buffer
	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"version", "--json"})
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&bytes.Buffer{})

	// when
	err := rootCmd.Execute()

	// then
	if err != nil {
		t.Fatalf("version --json failed: %v", err)
	}

	var info map[string]string
	if jsonErr := json.Unmarshal(stdout.Bytes(), &info); jsonErr != nil {
		t.Fatalf("invalid JSON output: %v\nraw: %s", jsonErr, stdout.String())
	}

	for _, key := range []string{"version", "commit", "date", "go"} {
		if _, ok := info[key]; !ok {
			t.Errorf("expected key %q in JSON output", key)
		}
	}
}
