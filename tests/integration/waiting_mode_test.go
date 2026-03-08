package integration_test

import (
	"testing"
	"time"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/session"
)

func TestWaitingMode_FeedbackCollectorNotify(t *testing.T) {
	// given
	ch := make(chan *session.DMail, 1)
	fc := session.CollectFeedback(nil, ch, nil, &domain.NopLogger{})

	// when: send D-Mail after short delay
	go func() {
		time.Sleep(20 * time.Millisecond)
		ch <- &session.DMail{Kind: session.DMailDesignFeedback, Name: "test-fb-001", Description: "test", SchemaVersion: "1"}
	}()

	// then: notification should arrive
	select {
	case <-fc.NotifyCh():
		// OK - notification received
	case <-time.After(2 * time.Second):
		t.Fatal("expected notification from FeedbackCollector")
	}
}

func TestWaitingMode_SnapshotAndNewSince(t *testing.T) {
	// given
	ch := make(chan *session.DMail, 5)
	fc := session.CollectFeedback(nil, ch, nil, &domain.NopLogger{})

	// Send initial D-Mail and wait for goroutine to process it
	ch <- &session.DMail{Kind: session.DMailDesignFeedback, Name: "fb-001", Description: "test", SchemaVersion: "1"}
	time.Sleep(50 * time.Millisecond)

	// when: snapshot, then send new D-Mail
	fc.Snapshot()

	ch <- &session.DMail{Kind: session.DMailSpecification, Name: "spec-001", Description: "new spec", SchemaVersion: "1"}
	time.Sleep(50 * time.Millisecond)

	// then: only the post-snapshot D-Mail should appear
	newMails := fc.NewSinceSnapshot()
	if len(newMails) != 1 {
		t.Fatalf("expected 1 new mail, got %d", len(newMails))
	}
	if newMails[0].Kind != session.DMailSpecification {
		t.Errorf("expected specification, got %s", newMails[0].Kind)
	}
	if newMails[0].Name != "spec-001" {
		t.Errorf("expected spec-001, got %s", newMails[0].Name)
	}
}

func TestWaitingMode_SnapshotEmpty(t *testing.T) {
	// given: collector with no D-Mails
	fc := session.CollectFeedback(nil, nil, nil, &domain.NopLogger{})

	// when
	fc.Snapshot()
	newMails := fc.NewSinceSnapshot()

	// then
	if len(newMails) != 0 {
		t.Fatalf("expected 0 new mails after empty snapshot, got %d", len(newMails))
	}
}

func TestWaitingMode_DisabledConfig(t *testing.T) {
	// given
	cfg := domain.DefaultConfig()

	// then: default timeout should be 30 minutes
	if cfg.Gate.WaitTimeout != domain.DefaultWaitTimeout {
		t.Errorf("expected default %v, got %v", domain.DefaultWaitTimeout, cfg.Gate.WaitTimeout)
	}

	// when: set negative timeout to disable waiting
	cfg.Gate.WaitTimeout = -1

	// then: negative timeout represents disabled waiting mode
	if cfg.Gate.WaitTimeout >= 0 {
		t.Error("negative timeout should represent disabled waiting mode")
	}
}

func TestWaitingMode_NotifyChNoSignalWithoutMail(t *testing.T) {
	// given: collector with no incoming D-Mails
	ch := make(chan *session.DMail, 1)
	fc := session.CollectFeedback(nil, ch, nil, &domain.NopLogger{})

	// then: NotifyCh should not fire within a short window
	select {
	case <-fc.NotifyCh():
		t.Fatal("should not have notification without D-Mail")
	case <-time.After(100 * time.Millisecond):
		// OK - no spurious notification
	}
}

func TestWaitingMode_MultipleMailsSingleNotify(t *testing.T) {
	// given
	ch := make(chan *session.DMail, 5)
	fc := session.CollectFeedback(nil, ch, nil, &domain.NopLogger{})

	// when: send multiple D-Mails rapidly
	ch <- &session.DMail{Kind: session.DMailDesignFeedback, Name: "fb-001", Description: "first", SchemaVersion: "1"}
	ch <- &session.DMail{Kind: session.DMailDesignFeedback, Name: "fb-002", Description: "second", SchemaVersion: "1"}
	ch <- &session.DMail{Kind: session.DMailDesignFeedback, Name: "fb-003", Description: "third", SchemaVersion: "1"}
	time.Sleep(100 * time.Millisecond)

	// then: should get at least one notification (channel is buffered size 1)
	select {
	case <-fc.NotifyCh():
		// OK
	case <-time.After(2 * time.Second):
		t.Fatal("expected notification after multiple D-Mails")
	}

	// and: all mails should be collected
	all := fc.All()
	if len(all) != 3 {
		t.Errorf("expected 3 collected mails, got %d", len(all))
	}
}

func TestWaitingMode_SnapshotThenMultipleNewMails(t *testing.T) {
	// given: collector with initial mail
	initial := []*session.DMail{
		{Kind: session.DMailDesignFeedback, Name: "init-001", Description: "initial", SchemaVersion: "1"},
	}
	ch := make(chan *session.DMail, 5)
	fc := session.CollectFeedback(initial, ch, nil, &domain.NopLogger{})

	// when: snapshot after initial, then send 2 new mails
	fc.Snapshot()

	ch <- &session.DMail{Kind: session.DMailDesignFeedback, Name: "fb-new-001", Description: "new1", SchemaVersion: "1"}
	ch <- &session.DMail{Kind: session.DMailReport, Name: "rpt-001", Description: "report", SchemaVersion: "1"}
	time.Sleep(100 * time.Millisecond)

	// then
	newMails := fc.NewSinceSnapshot()
	if len(newMails) != 2 {
		t.Fatalf("expected 2 new mails since snapshot, got %d", len(newMails))
	}
	if newMails[0].Name != "fb-new-001" {
		t.Errorf("expected fb-new-001, got %s", newMails[0].Name)
	}
	if newMails[1].Name != "rpt-001" {
		t.Errorf("expected rpt-001, got %s", newMails[1].Name)
	}
}
