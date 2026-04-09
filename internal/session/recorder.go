package session

import (
	"context"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/usecase/port"
)

// LoggingRecorder wraps a Recorder and logs errors instead of propagating them.
// This ensures callers never need to handle Record errors at every call site.
type LoggingRecorder struct {
	inner  port.Recorder
	logger domain.Logger
}

// NewLoggingRecorder creates a LoggingRecorder that wraps the given Recorder.
// If inner is nil, NopRecorder is used to prevent panics.
func NewLoggingRecorder(inner port.Recorder, logger domain.Logger) *LoggingRecorder {
	if inner == nil {
		inner = port.NopRecorder{}
	}
	return &LoggingRecorder{inner: inner, logger: logger}
}

// Record delegates to the inner Recorder. On error, it logs a warning and returns nil.
func (r *LoggingRecorder) Record(ctx context.Context, ev domain.Event) error {
	if err := r.inner.Record(ctx, ev); err != nil {
		r.logger.Warn("record event %s: %v", ev.Type, err)
	}
	return nil
}

// DispatchingRecorder wraps a Recorder and dispatches events to an EventDispatcher.
// Record is delegated to inner first; then the event is dispatched best-effort.
type DispatchingRecorder struct {
	inner      port.Recorder
	dispatcher port.EventDispatcher
	logger     domain.Logger
}

// NewDispatchingRecorder creates a DispatchingRecorder.
// If dispatcher is nil, Record simply delegates to inner.
func NewDispatchingRecorder(inner port.Recorder, dispatcher port.EventDispatcher, logger domain.Logger) *DispatchingRecorder {
	return &DispatchingRecorder{inner: inner, dispatcher: dispatcher, logger: logger}
}

// Record delegates to the inner Recorder, then dispatches the event best-effort.
func (r *DispatchingRecorder) Record(ctx context.Context, ev domain.Event) error {
	if err := r.inner.Record(ctx, ev); err != nil {
		return err
	}
	if r.dispatcher != nil {
		if dispatchErr := r.dispatcher.Dispatch(ctx, ev); dispatchErr != nil {
			if r.logger != nil {
				r.logger.Warn("policy dispatch %s: %v", ev.Type, dispatchErr)
			}
		}
	}
	return nil
}
