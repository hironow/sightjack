package usecase

import (
	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/session"
)

// GetStatus collects operational status from the event store and filesystem.
func GetStatus(baseDir string) domain.StatusReport {
	return session.Status(baseDir)
}
