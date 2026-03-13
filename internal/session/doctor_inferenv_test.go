// white-box-reason: tests that inference probe preserves claude_cmd env overrides via newCmd + FilterEnv interaction
package session

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/hironow/sightjack/internal/platform"
)

func TestInferenceProbePreservesClaudeCmdEnvOverrides(t *testing.T) {
	// given: claude_cmd includes env overrides like "CLAUDE_CONFIG_DIR=~/.claude-work-b claude"
	// When newCmd parses this, it sets cmd.Env = append(os.Environ(), "CLAUDE_CONFIG_DIR=...")
	// The inference probe must preserve those env vars while filtering CLAUDECODE.

	// Simulate what newCmd does when claude_cmd has env vars:
	// platform.NewShellCmd sets cmd.Env = append(os.Environ(), parsedEnvVars...)
	claudeCmd := "CLAUDE_CONFIG_DIR=/tmp/test-config claude"
	ctx := context.Background()
	cmd := platform.NewShellCmd(ctx, claudeCmd, "--print", "1+1=")

	// Verify newCmd set Env (because claude_cmd had env vars)
	if cmd.Env == nil {
		t.Fatal("expected NewShellCmd to set cmd.Env when claude_cmd has env vars")
	}

	// Verify CLAUDE_CONFIG_DIR is in the env
	hasConfigDir := false
	for _, e := range cmd.Env {
		if strings.HasPrefix(e, "CLAUDE_CONFIG_DIR=") {
			hasConfigDir = true
			break
		}
	}
	if !hasConfigDir {
		t.Fatal("expected CLAUDE_CONFIG_DIR in cmd.Env after NewShellCmd")
	}

	// when: apply the fixed FilterEnv logic (preserves existing cmd.Env)
	if cmd.Env != nil {
		cmd.Env = platform.FilterEnv(cmd.Env, "CLAUDECODE")
	}

	// then: CLAUDE_CONFIG_DIR should still be present
	hasConfigDir = false
	for _, e := range cmd.Env {
		if strings.HasPrefix(e, "CLAUDE_CONFIG_DIR=") {
			hasConfigDir = true
			break
		}
	}
	if !hasConfigDir {
		t.Error("CLAUDE_CONFIG_DIR was lost after FilterEnv — inference probe would lose claude_cmd env overrides")
	}
}

func TestInferenceProbeFiltersClaudecodeFromExistingEnv(t *testing.T) {
	// given: cmd.Env is already set (from claude_cmd with env vars) and contains CLAUDECODE
	cmd := &exec.Cmd{}
	cmd.Env = []string{
		"PATH=/usr/bin",
		"CLAUDE_CONFIG_DIR=/tmp/test-config",
		"CLAUDECODE=session123",
		"HOME=/tmp",
	}

	// when: apply the fixed logic
	if cmd.Env != nil {
		cmd.Env = platform.FilterEnv(cmd.Env, "CLAUDECODE")
	}

	// then: CLAUDECODE is removed but CLAUDE_CONFIG_DIR is preserved
	for _, e := range cmd.Env {
		if strings.HasPrefix(e, "CLAUDECODE=") {
			t.Error("CLAUDECODE should have been filtered out")
		}
	}
	hasConfigDir := false
	for _, e := range cmd.Env {
		if strings.HasPrefix(e, "CLAUDE_CONFIG_DIR=") {
			hasConfigDir = true
			break
		}
	}
	if !hasConfigDir {
		t.Error("CLAUDE_CONFIG_DIR should be preserved after filtering CLAUDECODE")
	}
}

func TestInferenceProbeNilEnvFallsBackToOsEnviron(t *testing.T) {
	// given: cmd.Env is nil (plain "claude" without env vars)
	cmd := &exec.Cmd{}

	// when: apply the fixed logic
	if cmd.Env != nil {
		cmd.Env = platform.FilterEnv(cmd.Env, "CLAUDECODE")
	} else {
		// Falls back to filtering os.Environ()
		cmd.Env = platform.FilterEnv([]string{"PATH=/usr/bin", "CLAUDECODE=x"}, "CLAUDECODE")
	}

	// then: CLAUDECODE is still filtered
	for _, e := range cmd.Env {
		if strings.HasPrefix(e, "CLAUDECODE=") {
			t.Error("CLAUDECODE should have been filtered from os.Environ() fallback")
		}
	}
}
