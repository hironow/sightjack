package session

import (
	"testing"
	"time"

	"github.com/hironow/sightjack/internal/domain"
)

func TestFeedbackCollector_NotifyCh(t *testing.T) {
	// given
	ch := make(chan *DMail, 1)
	fc := CollectFeedback(nil, ch, nil, &domain.NopLogger{})

	// NotifyCh should not fire yet
	select {
	case <-fc.NotifyCh():
		t.Fatal("should not have notification yet")
	default:
	}

	// when: send a D-Mail
	ch <- &DMail{Kind: DMailDesignFeedback, Name: "test-001"}
	// Give goroutine time to process
	time.Sleep(100 * time.Millisecond)

	// then: wait for notification
	select {
	case <-fc.NotifyCh():
		// OK
	case <-time.After(time.Second):
		t.Fatal("expected notification")
	}
}

func TestFeedbackCollector_NotifyCh_multipleDoesNotBlock(t *testing.T) {
	// given
	ch := make(chan *DMail, 3)
	fc := CollectFeedback(nil, ch, nil, &domain.NopLogger{})

	// when: send multiple D-Mails rapidly
	ch <- &DMail{Kind: DMailDesignFeedback, Name: "test-001"}
	ch <- &DMail{Kind: DMailDesignFeedback, Name: "test-002"}
	ch <- &DMail{Kind: DMailDesignFeedback, Name: "test-003"}
	time.Sleep(100 * time.Millisecond)

	// then: should get at least one notification without blocking
	select {
	case <-fc.NotifyCh():
		// OK
	case <-time.After(time.Second):
		t.Fatal("expected notification")
	}
}
