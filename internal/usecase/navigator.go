package usecase

import (
	"time"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/session"
)

// RenderNavigator renders a scan result as a navigator view.
func RenderNavigator(result *domain.ScanResult, projectName string) string {
	return session.RenderNavigator(result, projectName)
}

// RenderMatrixNavigator renders a matrix navigator view with waves.
func RenderMatrixNavigator(result *domain.ScanResult, projectName string, waves []domain.Wave, adrCount int, lastScanned *time.Time, strictnessLevel string, shibitoCount int) string {
	return session.RenderMatrixNavigator(result, projectName, waves, adrCount, lastScanned, strictnessLevel, shibitoCount)
}

// WriteScanResult serializes a ScanResult to a JSON file.
func WriteScanResult(path string, result *domain.ScanResult) error {
	return session.WriteScanResult(path, result)
}

// EventStorePath returns the filesystem path for a session's event store.
func EventStorePath(baseDir, sessionID string) string {
	return session.EventStorePath(baseDir, sessionID)
}
