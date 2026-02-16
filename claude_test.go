package sightjack

import (
	"os"
	"testing"
)

func TestBuildClaudeArgs(t *testing.T) {
	cfg := &Config{
		Claude: ClaudeConfig{
			Command: "claude",
			Model:   "opus",
		},
	}
	prompt := "Analyze these issues"

	args := BuildClaudeArgs(cfg, prompt)

	expected := []string{"--print", "--model", "opus", "-p", "Analyze these issues"}
	if len(args) != len(expected) {
		t.Fatalf("expected %d args, got %d: %v", len(expected), len(args), args)
	}
	for i, e := range expected {
		if args[i] != e {
			t.Errorf("arg[%d]: expected %q, got %q", i, e, args[i])
		}
	}
}

func TestBuildClaudeArgs_NoModel(t *testing.T) {
	cfg := &Config{
		Claude: ClaudeConfig{
			Command: "claude",
			Model:   "",
		},
	}
	prompt := "test prompt"

	args := BuildClaudeArgs(cfg, prompt)

	for _, a := range args {
		if a == "--model" {
			t.Error("--model should not be present when model is empty")
		}
	}
}

func TestRunClaudeDryRun(t *testing.T) {
	dir := t.TempDir()
	cfg := &Config{Claude: ClaudeConfig{Command: "claude"}}
	prompt := "test prompt content"
	outDir := dir + "/dryrun"

	err := RunClaudeDryRun(cfg, prompt, outDir)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(outDir + "/prompt.md")
	if err != nil {
		t.Fatalf("failed to read prompt file: %v", err)
	}
	if string(data) != prompt {
		t.Errorf("expected %q, got %q", prompt, string(data))
	}
}
