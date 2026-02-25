package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	sightjack "github.com/hironow/sightjack"
)

// EnsureMailDirs creates inbox/, outbox/, archive/ under .siren/.
func EnsureMailDirs(baseDir string) error {
	for _, sub := range []string{sightjack.InboxDir, sightjack.OutboxDir, sightjack.ArchiveDir} {
		if err := os.MkdirAll(sightjack.MailDir(baseDir, sub), 0755); err != nil {
			return fmt.Errorf("create %s dir: %w", sub, err)
		}
	}
	return nil
}

// WriteGitIgnore writes a .gitignore inside .siren/ that excludes ephemeral
// files (events/ and .run/) from version control.
// The write is idempotent — the file is always overwritten with the canonical content.
func WriteGitIgnore(baseDir string) error {
	content := "events/\n.run/\ninbox/\noutbox/\n"
	path := filepath.Join(baseDir, sightjack.StateDir, ".gitignore")
	return os.WriteFile(path, []byte(content), 0644)
}

// EnsureScanDir creates the scan directory for a session and returns its path.
// It also writes .siren/.gitignore as a best-effort side effect.
func EnsureScanDir(baseDir, sessionID string) (string, error) {
	dir := sightjack.ScanDir(baseDir, sessionID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create scan dir: %w", err)
	}
	// Best-effort: ensure .gitignore exists for first-run coverage.
	_ = WriteGitIgnore(baseDir)
	return dir, nil
}

// WriteScanResult serializes a ScanResult to a JSON file for session resume caching.
func WriteScanResult(path string, result *sightjack.ScanResult) error {
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
func LoadScanResult(path string) (*sightjack.ScanResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read scan result: %w", err)
	}
	var result sightjack.ScanResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse scan result: %w", err)
	}
	return &result, nil
}

// CanResume checks whether a saved session state supports resumption.
// It returns false when the cached ScanResult path is empty (e.g. v0.4
// state files) or the file no longer exists on disk.
func CanResume(state *sightjack.SessionState) bool {
	if state.ScanResultPath == "" {
		return false
	}
	if len(state.Waves) == 0 {
		return false
	}
	_, err := os.Stat(state.ScanResultPath)
	return err == nil
}
