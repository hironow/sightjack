//go:build contract

// Package contract_test verifies that sightjack's hardcoded assumptions about
// external services remain valid. These tests require real service access and
// are excluded from normal CI runs.
package contract_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"sort"
	"strings"
	"testing"

	"github.com/hironow/sightjack/internal/session"
)

// TestLinearMCPAllowedTools_ContractValidity verifies that every tool name in
// LinearMCPAllowedTools actually exists in the Linear MCP server's tool list.
//
// This catches silent drift: if the Linear MCP plugin renames or removes a tool,
// sightjack's WithAllowedTools filter would silently exclude it, causing
// classification or wave generation to fail without any error message.
//
// Requires: claude CLI available, Linear MCP server configured.
// Run: go test -tags contract -run TestLinearMCPAllowedTools -v ./tests/contract/
func TestLinearMCPAllowedTools_ContractValidity(t *testing.T) {
	if _, err := exec.LookPath("claude"); err != nil {
		t.Skip("claude CLI not found, skipping contract test")
	}

	if os.Getenv("LINEAR_API_KEY") == "" && os.Getenv("LINEAR_TOKEN") == "" {
		t.Skip("LINEAR_API_KEY or LINEAR_TOKEN not set, skipping contract test")
	}

	// given: the hardcoded tool list from sightjack
	allowedTools := session.LinearMCPAllowedTools

	// when: query the actual Linear MCP server for available tools
	actualTools := listLinearMCPTools(t)

	// then: every tool in LinearMCPAllowedTools must exist in the actual server
	missing := findMissing(allowedTools, actualTools)
	if len(missing) > 0 {
		t.Errorf("LinearMCPAllowedTools contains %d tool(s) not found in Linear MCP server:\n%s\n\n"+
			"This means sightjack's tool allowlist is out of sync with the MCP server.\n"+
			"Update LinearMCPAllowedTools in internal/session/claude.go",
			len(missing), strings.Join(missing, "\n"))
	}

	// also check: are there new Linear tools that sightjack doesn't know about?
	unknown := findMissing(filterLinearTools(actualTools), allowedTools)
	if len(unknown) > 0 {
		t.Logf("INFO: Linear MCP server has %d tool(s) not in LinearMCPAllowedTools:\n%s\n"+
			"Consider adding these if sightjack needs them.",
			len(unknown), strings.Join(unknown, "\n"))
	}
}

func listLinearMCPTools(t *testing.T) []string {
	t.Helper()

	cmd := exec.Command("claude", "mcp", "list", "--json")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return listLinearMCPToolsText(t)
	}

	var servers []struct {
		Name  string `json:"name"`
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &servers); err != nil {
		t.Logf("JSON parse failed, falling back to text: %v", err)
		return listLinearMCPToolsText(t)
	}

	var tools []string
	for _, server := range servers {
		for _, tool := range server.Tools {
			if strings.HasPrefix(tool.Name, "mcp__linear__") {
				tools = append(tools, tool.Name)
			}
		}
	}
	if len(tools) == 0 {
		t.Fatal("no Linear MCP tools found -- is the Linear MCP server configured?")
	}
	return tools
}

func listLinearMCPToolsText(t *testing.T) []string {
	t.Helper()
	cmd := exec.Command("claude", "mcp", "list")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("claude mcp list: %v", err)
	}

	var tools []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "mcp__linear__") {
			parts := strings.Fields(line)
			if len(parts) > 0 {
				tools = append(tools, parts[0])
			}
		}
	}
	if len(tools) == 0 {
		t.Fatal("no Linear MCP tools found in text output -- is the Linear MCP server configured?")
	}
	return tools
}

func findMissing(needles, haystack []string) []string {
	set := make(map[string]bool, len(haystack))
	for _, h := range haystack {
		set[h] = true
	}
	var missing []string
	for _, n := range needles {
		if !set[n] {
			missing = append(missing, n)
		}
	}
	sort.Strings(missing)
	return missing
}

func filterLinearTools(tools []string) []string {
	var filtered []string
	for _, t := range tools {
		if strings.HasPrefix(t, "mcp__linear__") {
			filtered = append(filtered, t)
		}
	}
	return filtered
}
