package domain

// Recorder records domain events during a session.
// session.go depends only on this interface, never on concrete implementations.
type Recorder interface {
	Record(eventType EventType, payload any) error
}

// HandoffResult tracks the outcome of a handoff for a single issue.
type HandoffResult struct {
	IssueID string // Linear issue identifier
	Status  string // "success", "failed", "skipped"
	Error   string // non-empty when Status is "failed"
}

// OutboxStore provides transactional outbox semantics for D-Mail delivery.
// Stage records intent in a durable store; Flush materializes staged items
// to the filesystem (archive/ + outbox/) using atomic writes.
type OutboxStore interface {
	// Stage atomically records a D-Mail for delivery. Idempotent: re-staging
	// the same name is a no-op.
	Stage(name string, data []byte) error

	// Flush writes all staged-but-unflushed D-Mails to archive/ and outbox/.
	// Returns the number of items flushed.
	Flush() (int, error)

	// Close releases database resources.
	Close() error
}
