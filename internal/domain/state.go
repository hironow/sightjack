package domain

import (
	"path/filepath"
)

const StateDir = ".siren"
const configFile = "config.yaml"

const (
	InboxDir   = "inbox"
	OutboxDir  = "outbox"
	ArchiveDir = "archive"
)

// MailDir returns the path to a mail subdirectory under the state root.
func MailDir(baseDir, sub string) string {
	return filepath.Join(baseDir, StateDir, sub)
}

// StateFormatVersion is the version string written into SessionState files.
// Centralised so that all code paths (scan, session, recovery) produce
// consistent state files.
const StateFormatVersion = "0.0.11"

// ConfigPath returns the path to the config file within .siren/.
func ConfigPath(baseDir string) string {
	return filepath.Join(baseDir, StateDir, configFile)
}

// ScanDir returns the path to the scan directory for a given session.
func ScanDir(baseDir, sessionID string) string {
	return filepath.Join(baseDir, StateDir, ".run", sessionID)
}

// RelativeScanResultPath converts an absolute scan result path to a path
// relative to baseDir for portable storage in event payloads. If the path
// is already relative, it is returned unchanged.
func RelativeScanResultPath(baseDir, absPath string) string {
	if !filepath.IsAbs(absPath) {
		return absPath
	}
	rel, err := filepath.Rel(baseDir, absPath)
	if err != nil {
		return absPath
	}
	return rel
}

// ResolveScanResultPath resolves a stored scan result path to an absolute path.
// If the stored path is already absolute (backwards compatibility with old events),
// it is returned as-is. If relative, it is joined with baseDir.
func ResolveScanResultPath(baseDir, storedPath string) string {
	if storedPath == "" {
		return ""
	}
	if filepath.IsAbs(storedPath) {
		return storedPath
	}
	return filepath.Join(baseDir, storedPath)
}
