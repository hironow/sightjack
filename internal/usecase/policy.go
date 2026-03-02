package usecase

import (
	"context"
	"fmt"

	sightjack "github.com/hironow/sightjack"
)

// PolicyHandler processes a domain event as part of a policy reaction.
// WHEN [EVENT] THEN [handler logic].
type PolicyHandler func(ctx context.Context, event sightjack.Event) error

// PolicyEngine dispatches domain events to registered policy handlers.
// This connects the POLICY registry (sightjack.Policies) to executable handlers.
type PolicyEngine struct {
	handlers map[sightjack.EventType][]PolicyHandler
	logger   *sightjack.Logger
}

// NewPolicyEngine creates a PolicyEngine. Pass nil logger for silent operation.
func NewPolicyEngine(logger *sightjack.Logger) *PolicyEngine {
	return &PolicyEngine{
		handlers: make(map[sightjack.EventType][]PolicyHandler),
		logger:   logger,
	}
}

// Register adds a handler for the given event type.
// Multiple handlers can be registered for the same event type.
func (e *PolicyEngine) Register(trigger sightjack.EventType, handler PolicyHandler) {
	e.handlers[trigger] = append(e.handlers[trigger], handler)
}

// Dispatch sends an event to all handlers registered for its type.
// Handlers execute sequentially; the first error stops dispatch.
func (e *PolicyEngine) Dispatch(ctx context.Context, event sightjack.Event) error {
	handlers, ok := e.handlers[event.Type]
	if !ok {
		return nil
	}
	for _, h := range handlers {
		if err := h(ctx, event); err != nil {
			return fmt.Errorf("policy dispatch %s: %w", event.Type, err)
		}
	}
	return nil
}
