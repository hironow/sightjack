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

// StateFormatVersion is the wire-format version embedded in SessionState JSON files.
// This is NOT the sightjack release version — it tracks the on-disk schema so that
// future readers can detect and migrate older state files.
//
// Compatibility contract (ADR 0013):
//   - Readers MUST accept all prior format versions (currently: "0.0.11", "1").
//   - Writers MUST always emit the current version.
//   - Bump only when the SessionState JSON structure changes incompatibly,
//     and add a migration path for every prior version.
const StateFormatVersion = "1"

// DMailSchemaVersion is the current D-Mail protocol schema version.
// All D-Mail generation paths MUST reference this constant (SPEC-003).
const DMailSchemaVersion = "1"

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
