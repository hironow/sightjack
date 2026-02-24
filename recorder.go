package sightjack

// Recorder records domain events during a session.
// session.go depends only on this interface, never on concrete implementations.
type Recorder interface {
	Record(eventType EventType, payload any) error
}

// NopRecorder is a no-op Recorder for dry-run mode and testing.
type NopRecorder struct{}

// Record always returns nil without recording anything.
func (NopRecorder) Record(EventType, any) error { return nil }

// LoggingRecorder wraps a Recorder and logs errors instead of propagating them.
// This ensures callers never need to handle Record errors at every call site.
type LoggingRecorder struct {
	inner  Recorder
	logger *Logger
}

// NewLoggingRecorder creates a LoggingRecorder that wraps the given Recorder.
func NewLoggingRecorder(inner Recorder, logger *Logger) *LoggingRecorder {
	return &LoggingRecorder{inner: inner, logger: logger}
}

// Record delegates to the inner Recorder. On error, it logs a warning and returns nil.
func (r *LoggingRecorder) Record(eventType EventType, payload any) error {
	if err := r.inner.Record(eventType, payload); err != nil {
		r.logger.Warn("record event %s: %v", eventType, err)
	}
	return nil
}
