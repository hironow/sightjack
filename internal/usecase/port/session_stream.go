// nosemgrep: structure.multiple-exported-interfaces-go -- session stream pub/sub triple (Publisher, Subscriber, Bus are a sealed pub/sub trio; Bus embeds Publisher and returns Subscriber; all three must be co-located for the interface composition to be legible) [permanent]
package port

import (
	"context"

	"github.com/hironow/sightjack/internal/domain"
)

// SessionStreamPublisher publishes session stream events to subscribers.
type SessionStreamPublisher interface { // nosemgrep: structure.multiple-exported-interfaces-go -- structure category drained in apr29-structure sweep; cohesive type family co-location is intentional [permanent]
	Publish(ctx context.Context, event domain.SessionStreamEvent)
}

// SessionStreamSubscriber receives session stream events.
type SessionStreamSubscriber interface { // nosemgrep: structure.multiple-exported-interfaces-go -- structure category drained in apr29-structure sweep; cohesive type family co-location is intentional [permanent]
	C() <-chan domain.SessionStreamEvent
	Close()
}

// SessionStreamBus manages pub/sub for session stream events.
type SessionStreamBus interface {
	SessionStreamPublisher
	Subscribe(bufSize int) SessionStreamSubscriber
	Close()
}
