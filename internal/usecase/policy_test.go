package usecase

import (
	"context"
	"fmt"
	"testing"
	"time"

	sightjack "github.com/hironow/sightjack"
)

func TestPolicyEngine_Dispatch_NoHandlers(t *testing.T) {
	// given
	engine := NewPolicyEngine(nil)
	ev, err := sightjack.NewEvent(sightjack.EventSessionStarted, sightjack.SessionStartedPayload{
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
	engine.Register(sightjack.EventWaveApproved, func(ctx context.Context, ev sightjack.Event) error {
		fired = true
		return nil
	})
	ev, err := sightjack.NewEvent(sightjack.EventWaveApproved, sightjack.WaveIdentityPayload{
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
		engine.Register(sightjack.EventWaveCompleted, func(ctx context.Context, ev sightjack.Event) error {
			count++
			return nil
		})
	}
	ev, err := sightjack.NewEvent(sightjack.EventWaveCompleted, sightjack.WaveCompletedPayload{
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

func TestPolicyEngine_HandlerError(t *testing.T) {
	// given
	engine := NewPolicyEngine(nil)
	engine.Register(sightjack.EventScanCompleted, func(ctx context.Context, ev sightjack.Event) error {
		return fmt.Errorf("handler failed")
	})
	ev, err := sightjack.NewEvent(sightjack.EventScanCompleted, sightjack.ScanCompletedPayload{
		Completeness: 0.5,
		ShibitoCount: 10,
	}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	// when
	dispatchErr := engine.Dispatch(context.Background(), ev)

	// then: first handler error stops dispatch
	if dispatchErr == nil {
		t.Fatal("expected error from handler")
	}
}

func TestPolicyEngine_UnmatchedEventType(t *testing.T) {
	// given: register for wave_approved only
	engine := NewPolicyEngine(nil)
	var fired bool
	engine.Register(sightjack.EventWaveApproved, func(ctx context.Context, ev sightjack.Event) error {
		fired = true
		return nil
	})
	ev, err := sightjack.NewEvent(sightjack.EventWaveRejected, sightjack.WaveIdentityPayload{
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
