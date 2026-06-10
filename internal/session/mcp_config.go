package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/hironow/sightjack/internal/domain"
)

// MCPConfig is the JSON structure for --mcp-config.
type MCPConfig struct { // nosemgrep: structure.multiple-exported-structs-go -- MCP config family (MCPConfig/MCPServerEntry) is a cohesive JSON wire-format pair for --mcp-config generation; splitting would fragment GenerateMCPConfig [permanent]
	MCPServers map[string]MCPServerEntry `json:"mcpServers"`
}

// MCPServerEntry defines a single MCP server.
type MCPServerEntry struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

// ClaudeSettingsPath returns the path to the .claude/settings.json file
// under the tool state directory.
func ClaudeSettingsPath(baseDir string) string {
	return filepath.Join(baseDir, domain.StateDir, ".claude", "settings.json")
}

// ClaudeSettingsExists reports whether .claude/settings.json exists.
func ClaudeSettingsExists(baseDir string) bool {
	_, err := os.Stat(ClaudeSettingsPath(baseDir))
	return err == nil
}

// GenerateClaudeSettings creates a minimal .claude/settings.json that
// disables all plugins for Claude subprocess isolation.
func GenerateClaudeSettings(baseDir string, force bool) (string, error) {
	path := ClaudeSettingsPath(baseDir)

	if !force {
		if _, err := os.Stat(path); err == nil {
			return path, fmt.Errorf(".claude/settings.json already exists (use --force to overwrite)")
		}
	}

	settings := map[string]any{
		"enabledPlugins": map[string]any{},
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal settings: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir: %w", err)
	}

	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return "", fmt.Errorf("write settings: %w", err)
	}

	return path, nil
}

// MCPConfigPath returns the path to the .mcp.json file.
func MCPConfigPath(baseDir string) string {
	return filepath.Join(baseDir, domain.StateDir, ".mcp.json")
}

// legacyMCPConfigPath returns the pre-rename path (.run/mcp-config.json)
// for backward compatibility during migration.
func legacyMCPConfigPath(baseDir string) string {
	return filepath.Join(baseDir, domain.StateDir, ".run", "mcp-config.json")
}

// ResolveMCPConfigPath returns the active MCP config path, preferring the
// new .mcp.json location and falling back to the legacy .run/mcp-config.json.
func ResolveMCPConfigPath(baseDir string) string {
	newPath := MCPConfigPath(baseDir)
	if _, err := os.Stat(newPath); err == nil {
		return newPath
	}
	legacyPath := legacyMCPConfigPath(baseDir)
	if _, err := os.Stat(legacyPath); err == nil {
		return legacyPath
	}
	return ""
}

// MCPConfigExists reports whether the .mcp.json file exists and is valid JSON.
func MCPConfigExists(baseDir string) (bool, error) {
	path := MCPConfigPath(baseDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	var cfg MCPConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return false, fmt.Errorf(".mcp.json invalid JSON: %w", err)
	}
	return true, nil
}

// GenerateMCPConfig creates a .mcp.json file.
// The generated config always includes this tool's MCP server used by
// human-initiated Claude Code sessions after the MCP pivot.
// If a legacy .run/mcp-config.json exists and the new file does not, the
// legacy content is migrated forward to preserve custom MCP server entries.
func GenerateMCPConfig(baseDir string, mode domain.TrackingMode, force bool) (string, error) {
	path := MCPConfigPath(baseDir)

	if !force {
		if _, err := os.Stat(path); err == nil {
			return path, fmt.Errorf(".mcp.json already exists (use --force to overwrite)")
		}
	}

	// Migrate legacy config if it exists and new file doesn't.
	// Skip migration on --force to allow clean regeneration from defaults.
	cfg := MCPConfig{
		MCPServers: make(map[string]MCPServerEntry),
	}
	if !force {
		legacyPath := legacyMCPConfigPath(baseDir)
		if legacyData, err := os.ReadFile(legacyPath); err == nil {
			if jsonErr := json.Unmarshal(legacyData, &cfg); jsonErr != nil {
				cfg.MCPServers = make(map[string]MCPServerEntry)
			}
		}
	}

	// Post-pivot configs attach this tool's own MCP server. Remove stale
	// Linear entries migrated from legacy configs; Linear tracking is no
	// longer a generated path.
	delete(cfg.MCPServers, "linear")
	cfg.MCPServers["sightjack"] = MCPServerEntry{
		Command: "sightjack",
		Args:    []string{"mcp"},
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal mcp-config: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("write mcp-config: %w", err)
	}

	// Also upsert the project-root .mcp.json (refs issue 0032 C5) so a
	// bare `claude` session in this project auto-attaches the server.
	// Merge-aware: sibling tools' entries survive.
	if _, err := UpsertRootMCPConfig(baseDir); err != nil {
		return "", fmt.Errorf("upsert root .mcp.json: %w", err)
	}

	return path, nil
}

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
