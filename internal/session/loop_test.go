package session

// white-box-reason: tests unexported classifyNewMails and report-triggered nextgen orchestration in waiting cycle

import (
	"fmt"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
)

func TestClassifyNewMails_Empty(t *testing.T) {
	t.Parallel()

	// when
	hasSpec, hasReport, issueIDs := classifyNewMails(nil)

	// then
	if hasSpec {
		t.Error("expected hasSpec=false for nil")
	}
	if hasReport {
		t.Error("expected hasReport=false for nil")
	}
	if len(issueIDs) != 0 {
		t.Errorf("expected empty issueIDs, got %v", issueIDs)
	}
}

func TestClassifyNewMails_SpecificationOnly(t *testing.T) {
	t.Parallel()

	// given
	mails := []*DMail{
		{Kind: DMailSpecification, Name: "spec-1"},
	}

	// when
	hasSpec, hasReport, issueIDs := classifyNewMails(mails)

	// then
	if !hasSpec {
		t.Error("expected hasSpec=true")
	}
	if hasReport {
		t.Error("expected hasReport=false")
	}
	if len(issueIDs) != 0 {
		t.Errorf("expected empty issueIDs, got %v", issueIDs)
	}
}

func TestClassifyNewMails_ReportWithIssues(t *testing.T) {
	t.Parallel()

	// given
	mails := []*DMail{
		{Kind: DMailReport, Name: "report-1", Issues: []string{"AUTH-100", "AUTH-101"}},
	}

	// when
	hasSpec, hasReport, issueIDs := classifyNewMails(mails)

	// then
	if hasSpec {
		t.Error("expected hasSpec=false")
	}
	if !hasReport {
		t.Error("expected hasReport=true")
	}
	if len(issueIDs) != 2 {
		t.Fatalf("expected 2 issueIDs, got %d", len(issueIDs))
	}
	if issueIDs[0] != "AUTH-100" || issueIDs[1] != "AUTH-101" {
		t.Errorf("issueIDs = %v, want [AUTH-100 AUTH-101]", issueIDs)
	}
}

func TestClassifyNewMails_ReportWithoutIssues(t *testing.T) {
	t.Parallel()

	// given: report D-Mail with no issues (edge case)
	mails := []*DMail{
		{Kind: DMailReport, Name: "report-empty"},
	}

	// when
	_, hasReport, issueIDs := classifyNewMails(mails)

	// then
	if !hasReport {
		t.Error("expected hasReport=true even without issues")
	}
	if len(issueIDs) != 0 {
		t.Errorf("expected empty issueIDs, got %v", issueIDs)
	}
}

func TestClassifyNewMails_MultipleReportsAggregateIssues(t *testing.T) {
	t.Parallel()

	// given
	mails := []*DMail{
		{Kind: DMailReport, Name: "report-1", Issues: []string{"AUTH-100"}},
		{Kind: DMailReport, Name: "report-2", Issues: []string{"BILL-200", "BILL-201"}},
	}

	// when
	_, _, issueIDs := classifyNewMails(mails)

	// then
	if len(issueIDs) != 3 {
		t.Fatalf("expected 3 issueIDs, got %d: %v", len(issueIDs), issueIDs)
	}
}

func TestClassifyNewMails_MixedKinds(t *testing.T) {
	t.Parallel()

	// given
	mails := []*DMail{
		{Kind: DMailSpecification, Name: "spec-1"},
		{Kind: DMailReport, Name: "report-1", Issues: []string{"AUTH-100"}},
		{Kind: DMailDesignFeedback, Name: "fb-1"},
	}

	// when
	hasSpec, hasReport, issueIDs := classifyNewMails(mails)

	// then
	if !hasSpec {
		t.Error("expected hasSpec=true")
	}
	if !hasReport {
		t.Error("expected hasReport=true")
	}
	if len(issueIDs) != 1 {
		t.Fatalf("expected 1 issueID, got %d", len(issueIDs))
	}
}

func TestClassifyNewMails_FeedbackOnly(t *testing.T) {
	t.Parallel()

	// given: only design-feedback D-Mails (no spec, no report)
	mails := []*DMail{
		{Kind: DMailDesignFeedback, Name: "fb-1"},
		{Kind: DMailDesignFeedback, Name: "fb-2"},
	}

	// when
	hasSpec, hasReport, issueIDs := classifyNewMails(mails)

	// then
	if hasSpec {
		t.Error("expected hasSpec=false")
	}
	if hasReport {
		t.Error("expected hasReport=false")
	}
	if len(issueIDs) != 0 {
		t.Errorf("expected empty issueIDs, got %v", issueIDs)
	}
}

// TestReportNextgenOrchestration_EndToEnd tests the full report → cluster → wave → nextgen
// decision path using only domain-layer functions (no session dependencies).
func TestReportNextgenOrchestration_EndToEnd(t *testing.T) {
	t.Parallel()

	// given: clusters with issues, waves with some completed
	clusters := []domain.ClusterScanResult{
		{
			Name:         "auth",
			Completeness: 0.6,
			Issues: []domain.IssueDetail{
				{Identifier: "AUTH-100"},
				{Identifier: "AUTH-101"},
			},
		},
		{
			Name:         "billing",
			Completeness: 0.4,
			Issues: []domain.IssueDetail{
				{Identifier: "BILL-200"},
			},
		},
	}
	waves := []domain.Wave{
		{ClusterName: "auth", ID: "w1", Status: "completed"},
		{ClusterName: "auth", ID: "w2", Status: "completed"},
		{ClusterName: "billing", ID: "w1", Status: "completed"},
	}

	// when: report D-Mails arrive with issue IDs
	reportMails := []*DMail{
		{Kind: DMailReport, Name: "paintress-report", Issues: []string{"AUTH-100", "BILL-200"}},
	}
	_, hasReport, reportIssueIDs := classifyNewMails(reportMails)

	// then: classification correct
	if !hasReport {
		t.Fatal("expected hasReport=true")
	}
	if len(reportIssueIDs) != 2 {
		t.Fatalf("expected 2 issueIDs, got %d", len(reportIssueIDs))
	}

	// when: map issues to clusters
	affected := domain.ClustersForIssueIDs(clusters, reportIssueIDs)

	// then: both clusters affected
	if len(affected) != 2 {
		t.Fatalf("expected 2 affected clusters, got %d", len(affected))
	}

	// when: check each cluster for nextgen eligibility
	for _, cluster := range affected {
		lastWave, ok := domain.LastCompletedWaveForCluster(waves, cluster.Name)
		if !ok {
			t.Errorf("expected last completed wave for cluster %q", cluster.Name)
			continue
		}
		needsMore := domain.NeedsMoreWaves(cluster, waves)

		// then: all waves completed + completeness < 0.95 → needs more waves
		if !needsMore {
			t.Errorf("expected NeedsMoreWaves=true for cluster %q (completeness=%.2f, all completed)", cluster.Name, cluster.Completeness)
		}
		if lastWave.ClusterName != cluster.Name {
			t.Errorf("lastWave.ClusterName = %q, want %q", lastWave.ClusterName, cluster.Name)
		}
	}
}

// TestReportNextgenOrchestration_SkipsHighCompleteness verifies that clusters
// at >= 0.95 completeness do NOT trigger nextgen, even when report D-Mails arrive.
func TestReportNextgenOrchestration_SkipsHighCompleteness(t *testing.T) {
	t.Parallel()

	// given: auth cluster near-complete
	clusters := []domain.ClusterScanResult{
		{
			Name:         "auth",
			Completeness: 0.96,
			Issues: []domain.IssueDetail{
				{Identifier: "AUTH-100"},
			},
		},
	}
	waves := []domain.Wave{
		{ClusterName: "auth", ID: "w1", Status: "completed"},
	}

	// when
	reportIssueIDs := []string{"AUTH-100"}
	affected := domain.ClustersForIssueIDs(clusters, reportIssueIDs)

	// then
	if len(affected) != 1 {
		t.Fatalf("expected 1 affected, got %d", len(affected))
	}
	if domain.NeedsMoreWaves(affected[0], waves) {
		t.Error("expected NeedsMoreWaves=false for 0.96 completeness")
	}
}

// TestReportNextgenOrchestration_SkipsNoCompletedWave verifies that clusters
// with no completed waves do NOT trigger nextgen (no lastWave to seed from).
func TestReportNextgenOrchestration_SkipsNoCompletedWave(t *testing.T) {
	t.Parallel()

	// given: cluster has only available waves, none completed
	clusters := []domain.ClusterScanResult{
		{
			Name:         "auth",
			Completeness: 0.3,
			Issues: []domain.IssueDetail{
				{Identifier: "AUTH-100"},
			},
		},
	}
	waves := []domain.Wave{
		{ClusterName: "auth", ID: "w1", Status: "available"},
	}

	// when
	affected := domain.ClustersForIssueIDs(clusters, []string{"AUTH-100"})
	_, ok := domain.LastCompletedWaveForCluster(waves, affected[0].Name)

	// then
	if ok {
		t.Error("expected no completed wave for auth")
	}
}

// TestReportNextgenOrchestration_SkipsRemainingAvailable verifies that clusters
// with remaining available waves do NOT trigger nextgen.
func TestReportNextgenOrchestration_SkipsRemainingAvailable(t *testing.T) {
	t.Parallel()

	// given
	clusters := []domain.ClusterScanResult{
		{
			Name:         "auth",
			Completeness: 0.5,
			Issues: []domain.IssueDetail{
				{Identifier: "AUTH-100"},
			},
		},
	}
	waves := []domain.Wave{
		{ClusterName: "auth", ID: "w1", Status: "completed"},
		{ClusterName: "auth", ID: "w2", Status: "available"}, // still available
	}

	// when
	affected := domain.ClustersForIssueIDs(clusters, []string{"AUTH-100"})

	// then: available wave remains → no nextgen needed
	if domain.NeedsMoreWaves(affected[0], waves) {
		t.Error("expected NeedsMoreWaves=false when available waves remain")
	}
}

// TestReportNextgenOrchestration_UnknownIssuesIgnored verifies that report D-Mails
// with issue IDs that don't match any cluster are silently ignored.
func TestReportNextgenOrchestration_UnknownIssuesIgnored(t *testing.T) {
	t.Parallel()

	// given
	clusters := []domain.ClusterScanResult{
		{
			Name:   "auth",
			Issues: []domain.IssueDetail{{Identifier: "AUTH-100"}},
		},
	}

	// when: report contains unknown issue IDs
	affected := domain.ClustersForIssueIDs(clusters, []string{"UNKNOWN-999"})

	// then
	if len(affected) != 0 {
		t.Errorf("expected 0 affected clusters for unknown issues, got %d", len(affected))
	}
}

// TestReportNextgenOrchestration_WaveCapReached verifies that clusters at max wave
// count do NOT trigger nextgen even when reports arrive.
func TestReportNextgenOrchestration_WaveCapReached(t *testing.T) {
	t.Parallel()

	// given: MaxWavesPerCluster (8) waves for auth
	clusters := []domain.ClusterScanResult{
		{
			Name:         "auth",
			Completeness: 0.5,
			Issues: []domain.IssueDetail{{Identifier: "AUTH-100"}},
		},
	}
	waves := make([]domain.Wave, domain.MaxWavesPerCluster)
	for i := range waves {
		waves[i] = domain.Wave{ClusterName: "auth", ID: fmt.Sprintf("w%d", i+1), Status: "completed"}
	}

	// when
	affected := domain.ClustersForIssueIDs(clusters, []string{"AUTH-100"})

	// then
	if domain.NeedsMoreWaves(affected[0], waves) {
		t.Error("expected NeedsMoreWaves=false when wave cap reached")
	}
}
