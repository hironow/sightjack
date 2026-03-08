package session

import (
	"context"
	"testing"
	"time"

	"github.com/hironow/sightjack/internal/domain"
)

func TestWaitForDMail_arrival(t *testing.T) {
	// given
	fc := &FeedbackCollector{notify: make(chan struct{}, 1)}
	fc.notify <- struct{}{} // simulate arrival

	// when
	ctx := context.Background()
	arrived, err := waitForDMail(ctx, fc, 5*time.Second, &domain.NopLogger{})

	// then
	if err != nil {
		t.Fatal(err)
	}
	if !arrived {
		t.Error("expected arrived=true")
	}
}

func TestWaitForDMail_timeout(t *testing.T) {
	// given
	fc := &FeedbackCollector{notify: make(chan struct{}, 1)}

	// when
	ctx := context.Background()
	arrived, err := waitForDMail(ctx, fc, 50*time.Millisecond, &domain.NopLogger{})

	// then
	if err != nil {
		t.Fatal(err)
	}
	if arrived {
		t.Error("expected arrived=false on timeout")
	}
}

func TestWaitForDMail_contextCancel(t *testing.T) {
	// given
	fc := &FeedbackCollector{notify: make(chan struct{}, 1)}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediate cancel

	// when
	arrived, err := waitForDMail(ctx, fc, 30*time.Minute, &domain.NopLogger{})

	// then
	if err != nil {
		t.Fatal(err)
	}
	if arrived {
		t.Error("expected arrived=false on cancel")
	}
}

func TestWaitForDMail_noTimeout(t *testing.T) {
	// given: timeout=0 means no timeout, only signal or context can exit
	fc := &FeedbackCollector{notify: make(chan struct{}, 1)}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// when
	arrived, err := waitForDMail(ctx, fc, 0, &domain.NopLogger{})

	// then
	if err != nil {
		t.Fatal(err)
	}
	if arrived {
		t.Error("expected arrived=false on context timeout")
	}
}
