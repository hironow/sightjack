package usecase
// white-box-reason: policy internals: tests unexported PolicyEngine constructor and Dispatch

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hironow/sightjack/internal/domain"
)

func TestPolicyEngine_Dispatch_NoHandlers(t *testing.T) {
	// given
	engine := NewPolicyEngine(nil)
	ev, err := domain.NewEvent(domain.EventSessionStarted, domain.SessionStartedPayload{
		Project:         "test-project",
		StrictnessLevel: "normal",
	}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	// when
	dispatchErr := engine.Dispatch(context.Background(), ev)

	// then: no handlers registered → no error
	if dispatchErr != nil {
		t.Fatalf("expected no error, got: %v", dispatchErr)
	}
}

func TestPolicyEngine_RegisterAndFire(t *testing.T) {
	// given
	engine := NewPolicyEngine(nil)
	var fired bool
	engine.Register(domain.EventWaveApproved, func(ctx context.Context, ev domain.Event) error {
		fired = true
		return nil
	})
	ev, err := domain.NewEvent(domain.EventWaveApproved, domain.WaveIdentityPayload{
		WaveID:      "wave-1",
		ClusterName: "cluster-a",
	}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	// when
	dispatchErr := engine.Dispatch(context.Background(), ev)

	// then
	if dispatchErr != nil {
		t.Fatalf("expected no error, got: %v", dispatchErr)
	}
	if !fired {
		t.Fatal("expected handler to fire")
	}
}

func TestPolicyEngine_MultipleHandlers(t *testing.T) {
	// given
	engine := NewPolicyEngine(nil)
	var count int
	for range 3 {
		engine.Register(domain.EventWaveCompleted, func(ctx context.Context, ev domain.Event) error {
			count++
			return nil
		})
	}
	ev, err := domain.NewEvent(domain.EventWaveCompleted, domain.WaveCompletedPayload{
		WaveID:      "wave-1",
		ClusterName: "cluster-a",
		Applied:     3,
		TotalCount:  3,
	}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	// when
	dispatchErr := engine.Dispatch(context.Background(), ev)

	// then
	if dispatchErr != nil {
		t.Fatalf("expected no error, got: %v", dispatchErr)
	}
	if count != 3 {
		t.Fatalf("expected 3 handlers to fire, got %d", count)
	}
}

func TestPolicyEngine_HandlerError_BestEffort(t *testing.T) {
	// given: two handlers — first fails, second succeeds
	engine := NewPolicyEngine(nil)
	var secondFired bool
	engine.Register(domain.EventScanCompleted, func(ctx context.Context, ev domain.Event) error {
		return fmt.Errorf("handler failed")
	})
	engine.Register(domain.EventScanCompleted, func(ctx context.Context, ev domain.Event) error {
		secondFired = true
		return nil
	})
	ev, err := domain.NewEvent(domain.EventScanCompleted, domain.ScanCompletedPayload{
		Completeness: 0.5,
		ShibitoCount: 10,
	}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	// when
	dispatchErr := engine.Dispatch(context.Background(), ev)

	// then: best-effort — error swallowed, all handlers execute, nil returned
	if dispatchErr != nil {
		t.Fatalf("expected nil (best-effort), got: %v", dispatchErr)
	}
	if !secondFired {
		t.Fatal("second handler should fire even after first handler error")
	}
}

func TestPolicyEngine_UnmatchedEventType(t *testing.T) {
	// given: register for wave_approved only
	engine := NewPolicyEngine(nil)
	var fired bool
	engine.Register(domain.EventWaveApproved, func(ctx context.Context, ev domain.Event) error {
		fired = true
		return nil
	})
	ev, err := domain.NewEvent(domain.EventWaveRejected, domain.WaveIdentityPayload{
		WaveID:      "wave-1",
		ClusterName: "cluster-a",
	}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	// when: dispatch a different event type
	dispatchErr := engine.Dispatch(context.Background(), ev)

	// then: handler should not fire
	if dispatchErr != nil {
		t.Fatalf("expected no error, got: %v", dispatchErr)
	}
	if fired {
		t.Fatal("handler should not fire for unmatched event type")
	}
}
