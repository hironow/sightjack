package session_test

import (
	"context"
	"testing"
	"time"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/session"
)

func TestStallDetector_EmitsAfterThreshold(t *testing.T) {
	// given — threshold=3, same structural error repeated 3 times
	store := &fakeOutboxStore{}
	detector := session.NewStallDetector(3, 30*time.Minute, &domain.NopLogger{})
	wave := domain.Wave{ID: "w1", ClusterName: "cluster1", Title: "Test Wave"}
	structuralErr := []string{"permission denied: /some/path"}

	// when — record 3 failures with same error
	detector.RecordFailure(context.Background(), store, wave, structuralErr)
	detector.RecordFailure(context.Background(), store, wave, structuralErr)
	detector.RecordFailure(context.Background(), store, wave, structuralErr)

	// then — stall D-Mail should have been staged
	if store.staged == 0 {
		t.Error("expected stall D-Mail to be staged after 3 structural failures")
	}
}

func TestStallDetector_DoesNotEmitBelowThreshold(t *testing.T) {
	// given
	store := &fakeOutboxStore{}
	detector := session.NewStallDetector(3, 30*time.Minute, &domain.NopLogger{})
	wave := domain.Wave{ID: "w1", ClusterName: "cluster1"}

	// when — only 2 failures
	detector.RecordFailure(context.Background(), store, wave, []string{"permission denied: /a"})
	detector.RecordFailure(context.Background(), store, wave, []string{"permission denied: /a"})

	// then
	if store.staged != 0 {
		t.Errorf("expected 0 staged, got %d", store.staged)
	}
}

func TestStallDetector_CooldownSuppressesDuplicate(t *testing.T) {
	// given — threshold=1 for easy triggering
	store := &fakeOutboxStore{}
	detector := session.NewStallDetector(1, 30*time.Minute, &domain.NopLogger{})
	wave := domain.Wave{ID: "w1", ClusterName: "cluster1"}
	errs := []string{"permission denied: /x"}

	// when — trigger twice
	detector.RecordFailure(context.Background(), store, wave, errs)
	detector.RecordFailure(context.Background(), store, wave, errs)

	// then — only 1 D-Mail (second suppressed by cooldown)
	if store.staged != 1 {
		t.Errorf("expected 1 staged (cooldown), got %d", store.staged)
	}
}

func TestStallDetector_SuccessResetsTracking(t *testing.T) {
	// given
	store := &fakeOutboxStore{}
	detector := session.NewStallDetector(3, 30*time.Minute, &domain.NopLogger{})
	wave := domain.Wave{ID: "w1", ClusterName: "cluster1"}
	errs := []string{"permission denied: /a"}

	// when — 2 failures, then success, then 2 more failures
	detector.RecordFailure(context.Background(), store, wave, errs)
	detector.RecordFailure(context.Background(), store, wave, errs)
	detector.RecordSuccess(wave) // reset
	detector.RecordFailure(context.Background(), store, wave, errs)
	detector.RecordFailure(context.Background(), store, wave, errs)

	// then — no stall (reset cleared the count)
	if store.staged != 0 {
		t.Errorf("expected 0 staged after reset, got %d", store.staged)
	}
}

// fakeOutboxStore counts Stage calls.
type fakeOutboxStore struct {
	staged int
}

func (f *fakeOutboxStore) Stage(_ context.Context, _ string, _ []byte) error {
	f.staged++
	return nil
}

func (f *fakeOutboxStore) Flush(_ context.Context) (int, error) { return 1, nil }
func (f *fakeOutboxStore) Close() error                         { return nil }
