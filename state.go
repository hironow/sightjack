package sightjack

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
