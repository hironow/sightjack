package usecase

// white-box-reason: emitter internals: tests unexported capturing store and event emission plumbing

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/usecase/port"
)

// testStore is a minimal EventStore for emit() testing.
type testStore struct {
	appendErr error
}

func (s *testStore) Append(_ context.Context, _ ...domain.Event) (domain.AppendResult, error) {
	return domain.AppendResult{}, s.appendErr
}
func (*testStore) LoadAll(_ context.Context) ([]domain.Event, domain.LoadResult, error) {
	return nil, domain.LoadResult{}, nil
}
func (*testStore) LoadSince(_ context.Context, _ time.Time) ([]domain.Event, domain.LoadResult, error) {
	return nil, domain.LoadResult{}, nil
}
func (*testStore) LoadAfterSeqNr(_ context.Context, _ uint64) ([]domain.Event, domain.LoadResult, error) {
	return nil, domain.LoadResult{}, nil
}
func (*testStore) LatestSeqNr(_ context.Context) (uint64, error) { return 0, nil }

func TestEmit_ReturnsStoreError(t *testing.T) {
	// given: emitter with a store that always fails
	agg := domain.NewSessionAggregate()
	emitter := NewSessionEventEmitter(
		context.Background(), agg, &testStore{appendErr: fmt.Errorf("store failure")}, nil,
		&domain.NopLogger{}, "test-session",
	)

	// when
	err := emitter.EmitStart("project", "fog", time.Now())

	// then: error should propagate from store
	if err == nil {
		t.Fatal("expected store error to propagate, got nil")
	}
}

func TestEmit_SucceedsWithWorkingStore(t *testing.T) {
	// given: emitter with a working store
	agg := domain.NewSessionAggregate()
	emitter := NewSessionEventEmitter(
		context.Background(), agg, &testStore{}, nil,
		&domain.NopLogger{}, "test-session",
	)

	// when
	err := emitter.EmitStart("project", "fog", time.Now())

	// then
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}

func TestEmit_DispatchErrorIsSwallowed(t *testing.T) {
	// given: emitter with working store but failing dispatcher
	agg := domain.NewSessionAggregate()
	emitter := NewSessionEventEmitter(
		context.Background(), agg, &testStore{}, &failingDispatcher{},
		&domain.NopLogger{}, "test-session",
	)

	// when: dispatcher fails but store succeeds
	err := emitter.EmitStart("project", "fog", time.Now())

	// then: no error (dispatch is best-effort)
	if err != nil {
		t.Fatalf("expected nil error (dispatch is best-effort), got: %v", err)
	}
}

type failingDispatcher struct{}

func (failingDispatcher) Dispatch(_ context.Context, _ domain.Event) error {
	return fmt.Errorf("dispatch failure")
}

// Compile-time check
var _ port.EventStore = (*testStore)(nil)
