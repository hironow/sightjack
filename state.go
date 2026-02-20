package sightjack

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const stateDir = ".siren"
const stateFile = "state.json"
const configFile = "config.yaml"

// StatePath returns the path to the state file within the given base directory.
func StatePath(baseDir string) string {
	return filepath.Join(baseDir, stateDir, stateFile)
}

// ConfigPath returns the path to the config file within .siren/.
func ConfigPath(baseDir string) string {
	return filepath.Join(baseDir, stateDir, configFile)
}

// WriteGitIgnore writes a .gitignore inside .siren/ that excludes ephemeral
// files (state.json and .run/) from version control.
// The write is idempotent — the file is always overwritten with the canonical content.
func WriteGitIgnore(baseDir string) error {
	content := "state.json\n.run/\ninbox/\noutbox/\n"
	path := filepath.Join(baseDir, stateDir, ".gitignore")
	return os.WriteFile(path, []byte(content), 0644)
}

// ScanDir returns the path to the scan directory for a given session.
func ScanDir(baseDir, sessionID string) string {
	return filepath.Join(baseDir, stateDir, ".run", sessionID)
}

// WriteState persists the session state as JSON to .siren/state.json.
func WriteState(baseDir string, state *SessionState) error {
	dir := filepath.Join(baseDir, stateDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	path := StatePath(baseDir)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write state: %w", err)
	}
	return nil
}

// ReadState loads the session state from .siren/state.json.
func ReadState(baseDir string) (*SessionState, error) {
	data, err := os.ReadFile(StatePath(baseDir))
	if err != nil {
		return nil, fmt.Errorf("read state: %w", err)
	}

	var state SessionState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parse state: %w", err)
	}
	return &state, nil
}

// EnsureScanDir creates the scan directory for a session and returns its path.
// It also writes .siren/.gitignore as a best-effort side effect.
func EnsureScanDir(baseDir, sessionID string) (string, error) {
	dir := ScanDir(baseDir, sessionID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create scan dir: %w", err)
	}
	// Best-effort: ensure .gitignore exists for first-run coverage.
	_ = WriteGitIgnore(baseDir)
	return dir, nil
}

// WriteScanResult serializes a ScanResult to a JSON file for session resume caching.
func WriteScanResult(path string, result *ScanResult) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal scan result: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write scan result: %w", err)
	}
	return nil
}

// LoadScanResult reads a cached ScanResult from a JSON file.
func LoadScanResult(path string) (*ScanResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read scan result: %w", err)
	}
	var result ScanResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse scan result: %w", err)
	}
	return &result, nil
}
