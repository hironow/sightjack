package sightjack

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const StateDir = ".siren"
const configFile = "config.yaml"

// StateFormatVersion is the version string written into SessionState files.
// Centralised so that all code paths (scan, session, recovery) produce
// consistent state files.
const StateFormatVersion = "0.0.11"

// ConfigPath returns the path to the config file within .siren/.
func ConfigPath(baseDir string) string {
	return filepath.Join(baseDir, StateDir, configFile)
}

// WriteGitIgnore writes a .gitignore inside .siren/ that excludes ephemeral
// files (events/ and .run/) from version control.
// The write is idempotent — the file is always overwritten with the canonical content.
func WriteGitIgnore(baseDir string) error {
	content := "events/\n.run/\ninbox/\noutbox/\n"
	path := filepath.Join(baseDir, StateDir, ".gitignore")
	return os.WriteFile(path, []byte(content), 0644)
}

// ScanDir returns the path to the scan directory for a given session.
func ScanDir(baseDir, sessionID string) string {
	return filepath.Join(baseDir, StateDir, ".run", sessionID)
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
