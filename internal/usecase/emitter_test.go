package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/usecase"
)

// fakeEventStore captures appended events for test verification.
type fakeEventStore struct {
	appended []domain.Event
	err      error // injected error for failure tests
}

func (s *fakeEventStore) Append(_ context.Context, events ...domain.Event) (domain.AppendResult, error) {
	if s.err != nil {
		return domain.AppendResult{}, s.err
	}
	s.appended = append(s.appended, events...)
	return domain.AppendResult{BytesWritten: len(events)}, nil
}

func (s *fakeEventStore) LoadAll(_ context.Context) ([]domain.Event, domain.LoadResult, error) {
	return nil, domain.LoadResult{}, nil
}

func (s *fakeEventStore) LoadSince(_ context.Context, _ time.Time) ([]domain.Event, domain.LoadResult, error) {
	return nil, domain.LoadResult{}, nil
}

func (s *fakeEventStore) LoadAfterSeqNr(_ context.Context, _ uint64) ([]domain.Event, domain.LoadResult, error) {
	return nil, domain.LoadResult{}, nil
}

func (s *fakeEventStore) LatestSeqNr(_ context.Context) (uint64, error) {
	return 0, nil
}

// fakeDispatcher captures dispatched events.
type fakeDispatcher struct {
	dispatched []domain.Event
}

func (d *fakeDispatcher) Dispatch(_ context.Context, event domain.Event) error {
	d.dispatched = append(d.dispatched, event)
	return nil
}

func TestSessionEventEmitter_StoresEvents(t *testing.T) {
	// given
	store := &fakeEventStore{}
	dispatcher := &fakeDispatcher{}
	agg := domain.NewSessionAggregate()
	emitter := usecase.NewSessionEventEmitter(context.Background(), agg, store, dispatcher, &domain.NopLogger{}, "test-session")

	// when
	err := emitter.EmitStart("test-project", "standard", time.Now())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(store.appended) != 1 {
		t.Errorf("expected 1 stored event, got %d", len(store.appended))
	}
	if len(dispatcher.dispatched) != 1 {
		t.Errorf("expected 1 dispatched event, got %d", len(dispatcher.dispatched))
	}
}

func TestSessionEventEmitter_BestEffort_StoreFailure(t *testing.T) {
	// given: store that always fails
	store := &fakeEventStore{err: errors.New("disk full")}
	dispatcher := &fakeDispatcher{}
	agg := domain.NewSessionAggregate()
	emitter := usecase.NewSessionEventEmitter(context.Background(), agg, store, dispatcher, &domain.NopLogger{}, "test-session")

	// when
	err := emitter.EmitStart("test-project", "standard", time.Now())

	// then: error is NOT propagated (best-effort semantics)
	if err != nil {
		t.Fatalf("expected nil error (best-effort), got: %v", err)
	}
}
