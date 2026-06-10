package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// This file is intentionally separate from mcp_config.go: that file is
// canonical-locked across the sibling tools (refs
// scripts/check_substrate_drift.sh), so the project-root wiring added
// for refs issue 0032 lives here and is wired from the cmd layer.

// RootMCPConfigPath returns the project-root .mcp.json path — the file
// Claude Code auto-discovers (project scope, pending-approval flow).
func RootMCPConfigPath(baseDir string) string {
	return filepath.Join(baseDir, ".mcp.json")
}

// UpsertRootMCPConfig merge-writes this tool's MCP server entry into
// the project-root .mcp.json (refs issue 0032, conformance constraint
// C5): existing entries from sibling tools (and any foreign top-level
// keys) are preserved so all five tap tools can share one root config
// for omni-sessions. Idempotent; the state-dir .mcp.json written by
// GenerateMCPConfig stays as the isolated `sessions enter` wiring.
func UpsertRootMCPConfig(baseDir string) (string, error) {
	path := RootMCPConfigPath(baseDir)

	root := map[string]json.RawMessage{}
	if data, err := os.ReadFile(path); err == nil {
		if jsonErr := json.Unmarshal(data, &root); jsonErr != nil {
			return "", fmt.Errorf("root .mcp.json invalid JSON (fix or remove it): %w", jsonErr)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("read root .mcp.json: %w", err)
	}

	servers := map[string]json.RawMessage{}
	if raw, ok := root["mcpServers"]; ok {
		if err := json.Unmarshal(raw, &servers); err != nil {
			return "", fmt.Errorf("root .mcp.json mcpServers invalid: %w", err)
		}
	}
	entry, err := json.Marshal(MCPServerEntry{Command: "sightjack", Args: []string{"mcp"}})
	if err != nil {
		return "", fmt.Errorf("marshal server entry: %w", err)
	}
	servers["sightjack"] = entry
	serversRaw, err := json.Marshal(servers)
	if err != nil {
		return "", fmt.Errorf("marshal mcpServers: %w", err)
	}
	root["mcpServers"] = serversRaw

	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal root .mcp.json: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return "", fmt.Errorf("write root .mcp.json: %w", err)
	}
	return path, nil
}
