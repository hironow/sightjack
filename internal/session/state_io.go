package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hironow/sightjack/internal/domain"
)

// sirenGitignoreEntries lists paths that must be gitignored in .siren/.
var sirenGitignoreEntries = []string{
	"events/",
	".run/",
	"inbox/",
	"outbox/",
	".otel.env",
}

// EnsureMailDirs creates inbox/, outbox/, archive/ under .siren/.
func EnsureMailDirs(baseDir string) error {
	for _, sub := range []string{domain.InboxDir, domain.OutboxDir, domain.ArchiveDir, "insights"} {
		if err := os.MkdirAll(domain.MailDir(baseDir, sub), 0755); err != nil {
			return fmt.Errorf("create %s dir: %w", sub, err)
		}
	}
	return nil
}

// WriteGitIgnore ensures a .gitignore inside .siren/ excludes ephemeral files
// from version control. Uses append-only pattern: existing user entries are preserved.
func WriteGitIgnore(baseDir string) error {
	return ensureGitignoreEntries(
		filepath.Join(baseDir, domain.StateDir, ".gitignore"),
		sirenGitignoreEntries,
	)
}

// ensureGitignoreEntries reads an existing .gitignore (if any) and appends
// any missing entries. Creates the file with all entries if it does not exist.
func ensureGitignoreEntries(path string, required []string) error {
	existing := ""
	if data, err := os.ReadFile(path); err == nil {
		existing = string(data)
	}

	var missing []string
	for _, entry := range required {
		if !strings.Contains(existing, entry) {
			missing = append(missing, entry)
		}
	}

	if len(missing) == 0 {
		return nil
	}

	if existing == "" {
		// New file: write all entries
		var buf strings.Builder
		for _, entry := range required {
			buf.WriteString(entry)
			buf.WriteByte('\n')
		}
		return os.WriteFile(path, []byte(buf.String()), 0o644)
	}

	// Existing file: append missing entries
	if !strings.HasSuffix(existing, "\n") {
		existing += "\n"
	}
	var buf strings.Builder
	buf.WriteString(existing)
	for _, entry := range missing {
		buf.WriteString(entry)
		buf.WriteByte('\n')
	}
	return os.WriteFile(path, []byte(buf.String()), 0o644)
}

// EnsureScanDir creates the scan directory for a session and returns its path.
// It also writes .siren/.gitignore as a best-effort side effect.
func EnsureScanDir(baseDir, sessionID string) (string, error) {
	dir := domain.ScanDir(baseDir, sessionID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create scan dir: %w", err)
	}
	// Best-effort: ensure .gitignore exists for first-run coverage.
	_ = WriteGitIgnore(baseDir)
	return dir, nil
}

// WriteScanResult serializes a ScanResult to a JSON file for session resume caching.
func WriteScanResult(path string, result *domain.ScanResult) error {
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
func LoadScanResult(path string) (*domain.ScanResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read scan result: %w", err)
	}
	var result domain.ScanResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse scan result: %w", err)
	}
	return &result, nil
}

// CanResume checks whether a saved session state supports resumption.
// It returns false when the cached ScanResult path is empty (e.g. v0.4
// state files) or the file no longer exists on disk.
// baseDir is used to resolve relative ScanResultPaths stored in newer events.
func CanResume(baseDir string, state *domain.SessionState) bool {
	if state.ScanResultPath == "" {
		return false
	}
	if len(state.Waves) == 0 {
		return false
	}
	resolved := domain.ResolveScanResultPath(baseDir, state.ScanResultPath)
	_, err := os.Stat(resolved)
	return err == nil
}
