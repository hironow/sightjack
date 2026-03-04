package usecase

import (
	"bufio"
	"context"
	"io"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/session"
)

// PreflightCheck verifies that required binaries exist in PATH.
func PreflightCheck(binaries ...string) error {
	return session.PreflightCheck(binaries...)
}

// SessionEventsDir returns the events directory for a specific session.
func SessionEventsDir(baseDir, sessionID string) string {
	return session.SessionEventsDir(baseDir, sessionID)
}

// NewEventStore creates an event store rooted at stateDir.
func NewEventStore(stateDir string) domain.EventStore {
	return session.NewEventStore(stateDir)
}

// NewSessionRecorder creates a recorder for the given session.
func NewSessionRecorder(store domain.EventStore, sessionID string) (domain.Recorder, error) {
	return session.NewSessionRecorder(store, sessionID)
}

// LoadLatestResumableState loads the latest session state that matches the predicate.
func LoadLatestResumableState(baseDir string, match func(*domain.SessionState) bool) (*domain.SessionState, string, error) {
	return session.LoadLatestResumableState(baseDir, match)
}

// CanResume checks whether a saved session state supports resumption.
func CanResume(baseDir string, state *domain.SessionState) bool {
	return session.CanResume(baseDir, state)
}

// PromptResume displays previous session info and asks the user to resume, start new, or re-scan.
func PromptResume(ctx context.Context, w io.Writer, s *bufio.Scanner, baseDir string, state *domain.SessionState) (domain.ResumeChoice, error) {
	return session.PromptResume(ctx, w, s, baseDir, state)
}
