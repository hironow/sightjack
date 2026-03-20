package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"go.opentelemetry.io/otel/attribute"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/eventsource"
	"github.com/hironow/sightjack/internal/platform"
	"github.com/hironow/sightjack/internal/usecase/port"
)

// EnsureRunDir creates the .run/ directory under stateDir if it does not exist.
// Call once before opening stores that write to .run/ (idempotent).
func EnsureRunDir(stateDir string) error {
	runDir := filepath.Join(stateDir, ".run")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return fmt.Errorf("ensure run dir: %w", err)
	}
	return nil
}

// stateDir converts a repo-root baseDir to the tool's state directory.
func stateDir(baseDir string) string {
	return filepath.Join(baseDir, domain.StateDir)
}

// SessionEventsDir returns the events directory for a specific session.
// Callers use this to compute the stateDir parameter for NewEventStore.
func SessionEventsDir(baseDir, sessionID string) string {
	return eventsource.EventStorePath(stateDir(baseDir), sessionID)
}

// NewEventStore creates an event store rooted at stateDir.
// eventsource is the event persistence adapter (AWS Event Sourcing pattern).
// cmd layer should use this instead of importing eventsource directly (ADR S0008).
func NewEventStore(stateDir string, logger domain.Logger) port.EventStore {
	raw := eventsource.NewFileEventStore(stateDir, logger)
	return NewSpanEventStore(raw)
}

// NewSessionRecorder creates a recorder for the given session.
func NewSessionRecorder(stateDir, sessionID string, logger domain.Logger) (port.Recorder, error) {
	raw := eventsource.NewFileEventStore(stateDir, logger)
	wrapped := NewSpanEventStore(raw)
	return eventsource.NewSessionRecorder(wrapped, sessionID)
}

// EventStorePath returns the filesystem path for a session's event store.
func EventStorePath(baseDir, sessionID string) string {
	return eventsource.EventStorePath(stateDir(baseDir), sessionID)
}

// LoadLatestState loads the most recent session state from event data.
func LoadLatestState(ctx context.Context, baseDir string) (*domain.SessionState, string, error) {
	ctx, span := platform.Tracer.Start(ctx, "eventsource.load_latest_state")
	defer span.End()
	_ = ctx
	state, id, err := eventsource.LoadLatestState(stateDir(baseDir))
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.String("error.stage", "eventsource.load_latest_state"))
	}
	return state, id, err
}

// LoadLatestResumableState loads the latest session state that matches the predicate.
func LoadLatestResumableState(ctx context.Context, baseDir string, match func(*domain.SessionState) bool) (*domain.SessionState, string, error) {
	ctx, span := platform.Tracer.Start(ctx, "eventsource.load_latest_resumable_state")
	defer span.End()
	_ = ctx
	state, id, err := eventsource.LoadLatestResumableState(stateDir(baseDir), match)
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.String("error.stage", "eventsource.load_latest_resumable_state"))
	}
	return state, id, err
}

// LoadAllEvents aggregates events from all session stores.
func LoadAllEvents(ctx context.Context, baseDir string) ([]domain.Event, error) {
	ctx, span := platform.Tracer.Start(ctx, "eventsource.load_all_events")
	defer span.End()
	_ = ctx
	events, loadResult, err := eventsource.LoadAllEventsAcrossSessions(stateDir(baseDir))
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.String("error.stage", "eventsource.load_all_events"))
	}
	if loadResult.SessionsFailed > 0 {
		span.SetAttributes(
			attribute.Int("sessions.loaded", loadResult.SessionsLoaded),
			attribute.Int("sessions.failed", loadResult.SessionsFailed),
		)
	}
	return events, err
}

// ListExpiredEventFiles returns event files older than the retention threshold.
func ListExpiredEventFiles(ctx context.Context, baseDir string, days int) ([]string, error) {
	_, span := platform.Tracer.Start(ctx, "eventsource.list_expired")
	defer span.End()
	files, err := eventsource.ListExpiredEventFiles(stateDir(baseDir), days)
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.String("error.stage", "eventsource.list_expired"))
	}
	span.SetAttributes(attribute.Int("event.count.out", len(files)))
	return files, err
}

// PruneEventFiles deletes the specified event files.
func PruneEventFiles(ctx context.Context, baseDir string, files []string) ([]string, error) {
	_, span := platform.Tracer.Start(ctx, "eventsource.prune")
	defer span.End()
	span.SetAttributes(attribute.Int("event.count.in", len(files)))
	deleted, err := eventsource.PruneEventFiles(stateDir(baseDir), files)
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.String("error.stage", "eventsource.prune"))
	}
	span.SetAttributes(attribute.Int("event.count.out", len(deleted)))
	return deleted, err
}
