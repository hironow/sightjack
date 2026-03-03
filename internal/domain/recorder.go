package domain

// NopRecorder is a no-op Recorder for dry-run mode and testing.
type NopRecorder struct{}

// Record always returns nil without recording anything.
func (NopRecorder) Record(EventType, any) error { return nil }
