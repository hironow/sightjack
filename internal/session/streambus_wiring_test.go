package session

// white-box-reason: tests that SetStreamBus propagates to NewClaudeAdapter

import (
	"context"
	"testing"
	"time"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/platform"
)

func TestStreamBusWiring_ClaudeAdapter(t *testing.T) {
	// given: process-wide StreamBus set
	bus := platform.NewInProcessSessionBus()
	defer bus.Close()
	sub := bus.Subscribe(16)
	defer sub.Close()

	old := sharedStreamBus
	SetStreamBus(bus)
	defer func() { sharedStreamBus = old }()

	// when: NewClaudeAdapter creates an adapter
	cfg := &domain.Config{ClaudeCmd: "nonexistent", Model: "test", TimeoutSec: 10}
	adapter := NewClaudeAdapter(cfg, &domain.NopLogger{})

	// then: StreamBus and ToolName are set
	if adapter.StreamBus == nil {
		t.Fatal("expected non-nil StreamBus on adapter")
	}
	if adapter.ToolName != "sightjack" {
		t.Errorf("expected ToolName=sightjack, got %q", adapter.ToolName)
	}

	// Verify bus is live by publishing and checking subscriber receives
	bus.Publish(context.Background(), domain.SessionStreamEvent{
		Tool:      "sightjack",
		Type: "session_end",
		Timestamp: time.Now(),
	})

	select {
	case ev := <-sub.C():
		if ev.Tool != "sightjack" {
			t.Errorf("expected Tool=sightjack, got %q", ev.Tool)
		}
	case <-time.After(time.Second):
		t.Fatal("subscriber did not receive event within timeout")
	}
}
