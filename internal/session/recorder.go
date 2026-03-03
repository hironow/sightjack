package session

import (
	"context"
	"time"

	"github.com/hironow/sightjack/internal/domain"
)

// LoggingRecorder wraps a Recorder and logs errors instead of propagating them.
// This ensures callers never need to handle Record errors at every call site.
type LoggingRecorder struct {
	inner  domain.Recorder
	logger *domain.Logger
}

// NewLoggingRecorder creates a LoggingRecorder that wraps the given Recorder.
// If inner is nil, NopRecorder is used to prevent panics.
func NewLoggingRecorder(inner domain.Recorder, logger *domain.Logger) *LoggingRecorder {
	if inner == nil {
		inner = domain.NopRecorder{}
	}
	return &LoggingRecorder{inner: inner, logger: logger}
}

// Record delegates to the inner Recorder. On error, it logs a warning and returns nil.
func (r *LoggingRecorder) Record(eventType domain.EventType, payload any) error {
	if err := r.inner.Record(eventType, payload); err != nil {
		r.logger.Warn("record event %s: %v", eventType, err)
	}
	return nil
}

// DispatchingRecorder wraps a Recorder and dispatches events to an EventDispatcher.
// Record is delegated to inner first; then an Event is constructed and dispatched best-effort.
type DispatchingRecorder struct {
	inner      domain.Recorder
	dispatcher domain.EventDispatcher
	logger     *domain.Logger
}

// NewDispatchingRecorder creates a DispatchingRecorder.
// If dispatcher is nil, Record simply delegates to inner.
func NewDispatchingRecorder(inner domain.Recorder, dispatcher domain.EventDispatcher, logger *domain.Logger) *DispatchingRecorder {
	return &DispatchingRecorder{inner: inner, dispatcher: dispatcher, logger: logger}
}

// Record delegates to the inner Recorder, then dispatches the event best-effort.
func (r *DispatchingRecorder) Record(eventType domain.EventType, payload any) error {
	if err := r.inner.Record(eventType, payload); err != nil {
		return err
	}
	if r.dispatcher != nil {
		ev, err := domain.NewEvent(eventType, payload, time.Now().UTC())
		if err != nil {
			if r.logger != nil {
				r.logger.Warn("policy dispatch build event %s: %v", eventType, err)
			}
			return nil
		}
		if dispatchErr := r.dispatcher.Dispatch(context.Background(), ev); dispatchErr != nil {
			if r.logger != nil {
				r.logger.Warn("policy dispatch %s: %v", eventType, dispatchErr)
			}
		}
	}
	return nil
}
