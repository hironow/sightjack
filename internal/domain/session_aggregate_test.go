package domain_test

import (
	"testing"
	"time"

	"github.com/hironow/sightjack/internal/domain"
)

func TestSessionAggregate_Start(t *testing.T) {
	// given
	agg := domain.NewSessionAggregate()

	// when
	ev, err := agg.Start("my-project", "standard", time.Now().UTC()) // nosemgrep: adr0003-otel-span-without-defer-end -- not an OTel span; domain aggregate method [permanent]

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != domain.EventSessionStartedV2 {
		t.Fatalf("expected session_started, got %s", ev.Type)
	}
}

func TestSessionAggregate_RecordScan(t *testing.T) {
	// given
	agg := domain.NewSessionAggregate()
	payload := domain.ScanCompletedPayload{
		Clusters:       []domain.ClusterState{{Name: "auth", Completeness: 0.3}},
		Completeness:   0.3,
		ShibitoCount:   5,
		ScanResultPath: "scan/result.json",
		LastScanned:    time.Now().UTC(),
	}

	// when
	ev, err := agg.RecordScan(payload, time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != domain.EventScanCompletedV2 {
		t.Fatalf("expected scan_completed, got %s", ev.Type)
	}
}

func TestSessionAggregate_UpdateCompleteness(t *testing.T) {
	// given
	agg := domain.NewSessionAggregate()

	// when
	ev, err := agg.UpdateCompleteness("auth", 0.6, 0.5, time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != domain.EventCompletenessUpdatedV2 {
		t.Fatalf("expected completeness_updated, got %s", ev.Type)
	}
}

func TestSessionAggregate_Resume(t *testing.T) {
	// given
	agg := domain.NewSessionAggregate()

	// when
	ev, err := agg.Resume("original-session-123", time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != domain.EventSessionResumedV2 {
		t.Fatalf("expected session_resumed, got %s", ev.Type)
	}
}

func TestSessionAggregate_Rescan(t *testing.T) {
	// given
	agg := domain.NewSessionAggregate()

	// when
	ev, err := agg.Rescan("original-session-456", time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != domain.EventSessionRescannedV2 {
		t.Fatalf("expected session_rescanned, got %s", ev.Type)
	}
}

func TestSessionAggregate_RecordWavesGenerated(t *testing.T) {
	// given
	agg := domain.NewSessionAggregate()

	// when
	ev, err := agg.RecordWavesGenerated(domain.WavesGeneratedPayload{
		Waves: []domain.WaveState{{ID: "w1", ClusterName: "auth"}},
	}, time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != domain.EventWavesGeneratedV2 {
		t.Fatalf("expected waves_generated, got %s", ev.Type)
	}
}

func TestSessionAggregate_ApproveWave(t *testing.T) {
	// given
	agg := domain.NewSessionAggregate()

	// when
	ev, err := agg.ApproveWave("w1", "auth", time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != domain.EventWaveApprovedV2 {
		t.Fatalf("expected wave_approved, got %s", ev.Type)
	}
}

func TestSessionAggregate_RejectWave(t *testing.T) {
	// given
	agg := domain.NewSessionAggregate()

	// when
	ev, err := agg.RejectWave("w1", "auth", time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != domain.EventWaveRejectedV2 {
		t.Fatalf("expected wave_rejected, got %s", ev.Type)
	}
}

func TestSessionAggregate_ModifyWave(t *testing.T) {
	// given
	agg := domain.NewSessionAggregate()

	// when
	ev, err := agg.ModifyWave(domain.WaveModifiedPayload{
		WaveID: "w1", ClusterName: "auth",
		UpdatedWave: domain.WaveState{ID: "w1", ClusterName: "auth"},
	}, time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != domain.EventWaveModifiedV2 {
		t.Fatalf("expected wave_modified, got %s", ev.Type)
	}
}

func TestSessionAggregate_ApplyWave(t *testing.T) {
	// given
	agg := domain.NewSessionAggregate()

	// when
	ev, err := agg.ApplyWave(domain.WaveAppliedPayload{
		WaveID: "w1", ClusterName: "auth", Applied: 3, TotalCount: 5,
	}, time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != domain.EventWaveAppliedV2 {
		t.Fatalf("expected wave_applied, got %s", ev.Type)
	}
}

func TestSessionAggregate_CompleteWave(t *testing.T) {
	// given
	agg := domain.NewSessionAggregate()

	// when
	ev, err := agg.CompleteWave(domain.WaveCompletedPayload{
		WaveID: "w1", ClusterName: "auth", Applied: 5, TotalCount: 5,
	}, time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != domain.EventWaveCompletedV2 {
		t.Fatalf("expected wave_completed, got %s", ev.Type)
	}
}

func TestSessionAggregate_AddNextGenWaves(t *testing.T) {
	// given
	agg := domain.NewSessionAggregate()

	// when
	ev, err := agg.AddNextGenWaves(domain.NextGenWavesAddedPayload{
		ClusterName: "auth",
		Waves:       []domain.WaveState{{ID: "w2", ClusterName: "auth"}},
	}, time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != domain.EventNextGenWavesAddedV2 {
		t.Fatalf("expected nextgen_waves_added, got %s", ev.Type)
	}
}

func TestSessionAggregate_ApplyReadyLabels(t *testing.T) {
	// given
	agg := domain.NewSessionAggregate()

	// when
	ev, err := agg.ApplyReadyLabels(domain.ReadyLabelsAppliedPayload{
		IssueIDs: []string{"ISSUE-1", "ISSUE-2"},
	}, time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != domain.EventReadyLabelsAppliedV2 {
		t.Fatalf("expected ready_labels_applied, got %s", ev.Type)
	}
}

func TestSessionAggregate_SendSpecification(t *testing.T) {
	// given
	agg := domain.NewSessionAggregate()

	// when
	ev, err := agg.SendSpecification("w1", "auth", time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != domain.EventSpecificationSentV2 {
		t.Fatalf("expected specification_sent, got %s", ev.Type)
	}
}

func TestSessionAggregate_SendReport(t *testing.T) {
	// given
	agg := domain.NewSessionAggregate()

	// when
	ev, err := agg.SendReport("w1", "auth", time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != domain.EventReportSentV2 {
		t.Fatalf("expected report_sent, got %s", ev.Type)
	}
}

func TestSessionAggregate_SendFeedback(t *testing.T) {
	// given
	agg := domain.NewSessionAggregate()

	// when
	ev, err := agg.SendFeedback("w1", "auth", time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != domain.EventFeedbackSentV2 {
		t.Fatalf("expected feedback_sent, got %s", ev.Type)
	}
}

func TestSessionAggregate_ReceiveFeedback(t *testing.T) {
	// given
	agg := domain.NewSessionAggregate()

	// when
	ev, err := agg.ReceiveFeedback(domain.FeedbackReceivedPayload{
		Kind:  "design-feedback",
		Name:  "design-feedback-batch",
		Count: 3,
	}, time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != domain.EventFeedbackReceivedV2 {
		t.Fatalf("expected feedback_received, got %s", ev.Type)
	}
}

func TestSessionAggregate_GenerateADR(t *testing.T) {
	// given
	agg := domain.NewSessionAggregate()

	// when
	ev, err := agg.GenerateADR(domain.ADRGeneratedPayload{
		ADRID: "0001", Title: "Use FastAPI",
	}, time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != domain.EventADRGeneratedV2 {
		t.Fatalf("expected adr_generated, got %s", ev.Type)
	}
}

func TestSessionAggregate_UnlockWaves(t *testing.T) {
	// given
	agg := domain.NewSessionAggregate()

	// when
	ev, err := agg.UnlockWaves([]string{"auth:w2", "auth:w3"}, time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != domain.EventWavesUnlockedV2 {
		t.Fatalf("expected waves_unlocked, got %s", ev.Type)
	}
}
