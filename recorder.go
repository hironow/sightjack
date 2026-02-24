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
