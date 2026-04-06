package integration_test

import (
	"sort"
	"testing"
	"time"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/harness"
	"github.com/hironow/sightjack/internal/session"
)

func TestWaitingMode_FeedbackCollectorNotify(t *testing.T) {
	// given
	ch := make(chan *domain.DMail, 1)
	fc := session.CollectFeedback(nil, ch, nil, &domain.NopLogger{})

	// when: send D-Mail after short delay
	go func() {
		time.Sleep(20 * time.Millisecond)
		ch <- &domain.DMail{Kind: domain.KindDesignFeedback, Name: "test-fb-001", Description: "test", SchemaVersion: "1"}
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
	ch := make(chan *domain.DMail, 5)
	fc := session.CollectFeedback(nil, ch, nil, &domain.NopLogger{})

	// Send initial D-Mail and wait for goroutine to process it
	ch <- &domain.DMail{Kind: domain.KindDesignFeedback, Name: "fb-001", Description: "test", SchemaVersion: "1"}
	time.Sleep(50 * time.Millisecond)

	// when: snapshot, then send new D-Mail
	fc.Snapshot()

	ch <- &domain.DMail{Kind: domain.KindSpecification, Name: "spec-001", Description: "new spec", SchemaVersion: "1"}
	time.Sleep(50 * time.Millisecond)

	// then: only the post-snapshot D-Mail should appear
	newMails := fc.NewSinceSnapshot()
	if len(newMails) != 1 {
		t.Fatalf("expected 1 new mail, got %d", len(newMails))
	}
	if newMails[0].Kind != domain.KindSpecification {
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
	if cfg.Gate.IdleTimeout != domain.DefaultIdleTimeout {
		t.Errorf("expected default %v, got %v", domain.DefaultIdleTimeout, cfg.Gate.IdleTimeout)
	}

	// when: set negative timeout to disable waiting
	cfg.Gate.IdleTimeout = -1

	// then: negative timeout represents disabled waiting mode
	if cfg.Gate.IdleTimeout >= 0 {
		t.Error("negative timeout should represent disabled waiting mode")
	}
}

func TestWaitingMode_NotifyChNoSignalWithoutMail(t *testing.T) {
	// given: collector with no incoming D-Mails
	ch := make(chan *domain.DMail, 1)
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
	ch := make(chan *domain.DMail, 5)
	fc := session.CollectFeedback(nil, ch, nil, &domain.NopLogger{})

	// when: send multiple D-Mails rapidly
	ch <- &domain.DMail{Kind: domain.KindDesignFeedback, Name: "fb-001", Description: "first", SchemaVersion: "1"}
	ch <- &domain.DMail{Kind: domain.KindDesignFeedback, Name: "fb-002", Description: "second", SchemaVersion: "1"}
	ch <- &domain.DMail{Kind: domain.KindDesignFeedback, Name: "fb-003", Description: "third", SchemaVersion: "1"}
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
	initial := []*domain.DMail{
		{Kind: domain.KindDesignFeedback, Name: "init-001", Description: "initial", SchemaVersion: "1"},
	}
	ch := make(chan *domain.DMail, 5)
	fc := session.CollectFeedback(initial, ch, nil, &domain.NopLogger{})

	// when: snapshot after initial, then send 2 new mails
	fc.Snapshot()

	ch <- &domain.DMail{Kind: domain.KindDesignFeedback, Name: "fb-new-001", Description: "new1", SchemaVersion: "1"}
	ch <- &domain.DMail{Kind: domain.KindReport, Name: "rpt-001", Description: "report", SchemaVersion: "1"}
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

// TestWaitingMode_ReportDMailTriggersClusterIdentification verifies the full
// integration path from report D-Mail arrival through FeedbackCollector to
// cluster identification and nextgen eligibility check.
func TestWaitingMode_ReportDMailTriggersClusterIdentification(t *testing.T) {
	// given: a FeedbackCollector with initial feedback, then report arrives
	ch := make(chan *domain.DMail, 5)
	initial := []*domain.DMail{
		{Kind: domain.KindDesignFeedback, Name: "fb-001", Description: "initial feedback", SchemaVersion: "1"},
	}
	fc := session.CollectFeedback(initial, ch, nil, &domain.NopLogger{})

	// Snapshot before waiting
	fc.Snapshot()

	// Report D-Mail arrives during waiting
	ch <- &domain.DMail{
		Kind:          domain.KindReport,
		Name:          "paintress-pr-created",
		Description:   "PR #42 created for auth issues",
		SchemaVersion: "1",
		Issues:        []string{"AUTH-100", "AUTH-101"},
	}
	time.Sleep(100 * time.Millisecond)

	// when: classify new mails since snapshot
	newMails := fc.NewSinceSnapshot()
	if len(newMails) != 1 {
		t.Fatalf("expected 1 new mail, got %d", len(newMails))
	}

	// then: report mail has correct kind and issues
	mail := newMails[0]
	if mail.Kind != domain.KindReport {
		t.Errorf("kind = %s, want report", mail.Kind)
	}
	if len(mail.Issues) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(mail.Issues))
	}

	// when: map issues to clusters
	clusters := []domain.ClusterScanResult{
		{
			Name:         "auth",
			Completeness: 0.5,
			Issues:       []domain.IssueDetail{{Identifier: "AUTH-100"}, {Identifier: "AUTH-101"}},
		},
		{
			Name:         "billing",
			Completeness: 0.3,
			Issues:       []domain.IssueDetail{{Identifier: "BILL-200"}},
		},
	}
	affected := harness.ClustersForIssueIDs(clusters, mail.Issues)

	// then: only auth cluster affected
	if len(affected) != 1 {
		t.Fatalf("expected 1 affected cluster, got %d", len(affected))
	}
	if affected[0].Name != "auth" {
		t.Errorf("affected cluster = %q, want auth", affected[0].Name)
	}
}

// TestWaitingMode_MultipleReportDMailsAcrossClusters tests that multiple report
// D-Mails arriving in the same waiting cycle correctly aggregate issue IDs
// and identify all affected clusters.
func TestWaitingMode_MultipleReportDMailsAcrossClusters(t *testing.T) {
	// given
	ch := make(chan *domain.DMail, 5)
	fc := session.CollectFeedback(nil, ch, nil, &domain.NopLogger{})

	fc.Snapshot()

	// Two report D-Mails from different tools
	ch <- &domain.DMail{
		Kind:          domain.KindReport,
		Name:          "paintress-auth",
		Description:   "PR for auth",
		SchemaVersion: "1",
		Issues:        []string{"AUTH-100"},
	}
	ch <- &domain.DMail{
		Kind:          domain.KindReport,
		Name:          "paintress-billing",
		Description:   "PR for billing",
		SchemaVersion: "1",
		Issues:        []string{"BILL-200"},
	}
	time.Sleep(100 * time.Millisecond)

	// when
	newMails := fc.NewSinceSnapshot()
	if len(newMails) != 2 {
		t.Fatalf("expected 2 new mails, got %d", len(newMails))
	}

	// Aggregate issue IDs (same logic as classifyNewMails)
	var issueIDs []string
	for _, m := range newMails {
		if m.Kind == domain.KindReport {
			issueIDs = append(issueIDs, m.Issues...)
		}
	}

	clusters := []domain.ClusterScanResult{
		{Name: "auth", Completeness: 0.6, Issues: []domain.IssueDetail{{Identifier: "AUTH-100"}}},
		{Name: "billing", Completeness: 0.4, Issues: []domain.IssueDetail{{Identifier: "BILL-200"}}},
		{Name: "infra", Completeness: 0.8, Issues: []domain.IssueDetail{{Identifier: "INFRA-300"}}},
	}
	affected := harness.ClustersForIssueIDs(clusters, issueIDs)

	// then: auth and billing affected, infra not
	if len(affected) != 2 {
		t.Fatalf("expected 2 affected clusters, got %d", len(affected))
	}
	names := []string{affected[0].Name, affected[1].Name}
	sort.Strings(names)
	if names[0] != "auth" || names[1] != "billing" {
		t.Errorf("affected = %v, want [auth billing]", names)
	}
}

// TestWaitingMode_ReportAndFeedbackMixed verifies that a mix of report and
// feedback D-Mails arriving in the same waiting cycle correctly separates
// report issues from feedback-only mails.
func TestWaitingMode_ReportAndFeedbackMixed(t *testing.T) {
	// given
	ch := make(chan *domain.DMail, 5)
	fc := session.CollectFeedback(nil, ch, nil, &domain.NopLogger{})

	fc.Snapshot()

	ch <- &domain.DMail{Kind: domain.KindDesignFeedback, Name: "fb-001", Description: "feedback", SchemaVersion: "1"}
	ch <- &domain.DMail{Kind: domain.KindReport, Name: "rpt-001", Description: "report", SchemaVersion: "1", Issues: []string{"AUTH-100"}}
	ch <- &domain.DMail{Kind: domain.KindDesignFeedback, Name: "fb-002", Description: "more feedback", SchemaVersion: "1"}
	time.Sleep(100 * time.Millisecond)

	// when
	newMails := fc.NewSinceSnapshot()

	// then: 3 total mails
	if len(newMails) != 3 {
		t.Fatalf("expected 3 new mails, got %d", len(newMails))
	}

	// Count by kind
	reports := 0
	feedback := 0
	var issueIDs []string
	for _, m := range newMails {
		switch m.Kind {
		case domain.KindReport:
			reports++
			issueIDs = append(issueIDs, m.Issues...)
		case domain.KindDesignFeedback:
			feedback++
		}
	}

	if reports != 1 {
		t.Errorf("expected 1 report, got %d", reports)
	}
	if feedback != 2 {
		t.Errorf("expected 2 feedback, got %d", feedback)
	}
	if len(issueIDs) != 1 || issueIDs[0] != "AUTH-100" {
		t.Errorf("issueIDs = %v, want [AUTH-100]", issueIDs)
	}
}

// TestWaitingMode_NextgenEligibilityAfterReport exercises the complete
// waiting-mode decision path: report arrives → clusters identified →
// NeedsMoreWaves check for each cluster.
func TestWaitingMode_NextgenEligibilityAfterReport(t *testing.T) {
	// given: two clusters, auth (all waves completed), billing (wave still available)
	clusters := []domain.ClusterScanResult{
		{Name: "auth", Completeness: 0.6, Issues: []domain.IssueDetail{{Identifier: "AUTH-100"}}},
		{Name: "billing", Completeness: 0.4, Issues: []domain.IssueDetail{{Identifier: "BILL-200"}}},
	}
	waves := []domain.Wave{
		{ClusterName: "auth", ID: "w1", Status: "completed"},
		{ClusterName: "auth", ID: "w2", Status: "completed"},
		{ClusterName: "billing", ID: "w1", Status: "completed"},
		{ClusterName: "billing", ID: "w2", Status: "available"}, // still has work
	}

	// when: report arrives for both clusters
	issueIDs := []string{"AUTH-100", "BILL-200"}
	affected := harness.ClustersForIssueIDs(clusters, issueIDs)

	// then
	authNeedsMore := false
	billingNeedsMore := false
	for _, c := range affected {
		lastWave, ok := harness.LastCompletedWaveForCluster(waves, c.Name)
		if !ok {
			continue
		}
		needs := harness.NeedsMoreWaves(c, waves)
		if c.Name == "auth" {
			authNeedsMore = needs
			if lastWave.ID != "w2" {
				t.Errorf("auth lastWave.ID = %q, want w2", lastWave.ID)
			}
		}
		if c.Name == "billing" {
			billingNeedsMore = needs
		}
	}

	// auth: all completed + completeness < 0.95 → needs more
	if !authNeedsMore {
		t.Error("expected auth NeedsMoreWaves=true (all completed, low completeness)")
	}
	// billing: has available wave → does NOT need more
	if billingNeedsMore {
		t.Error("expected billing NeedsMoreWaves=false (available wave remains)")
	}
}
