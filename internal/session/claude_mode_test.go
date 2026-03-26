package session_test

import (
	"testing"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/session"
)

func TestAllowedToolsForMode_WaveExcludesLinear(t *testing.T) {
	// when
	tools := session.AllowedToolsForMode(domain.ModeWave)

	// then: should contain base + GH tools but NOT Linear tools
	hasLinear := false
	for _, tool := range tools {
		if tool == "mcp__linear__list_issues" {
			hasLinear = true
			break
		}
	}
	if hasLinear {
		t.Error("Wave mode should not include Linear MCP tools")
	}
	if len(tools) == 0 {
		t.Error("Wave mode should include at least base tools")
	}
}

func TestAllowedToolsForMode_LinearIncludesAll(t *testing.T) {
	// when
	tools := session.AllowedToolsForMode(domain.ModeLinear)

	// then: should contain Linear tools
	hasLinear := false
	for _, tool := range tools {
		if tool == "mcp__linear__list_issues" {
			hasLinear = true
			break
		}
	}
	if !hasLinear {
		t.Error("Linear mode should include Linear MCP tools")
	}
	if len(tools) <= len(session.BaseAllowedTools)+len(session.GHAllowedTools) {
		t.Error("Linear mode should have more tools than base+GH")
	}
}
