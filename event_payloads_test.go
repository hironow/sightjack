package sightjack_test

import (
	"testing"
	"time"

	"github.com/hironow/sightjack"
)

func TestPayload_SessionStarted_RoundTrip(t *testing.T) {
	// given
	payload := sightjack.SessionStartedPayload{
		Project:         "my-project",
		StrictnessLevel: "fog",
	}
	event, err := sightjack.NewEvent(sightjack.EventSessionStarted, "s1", 1, payload)
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}

	// when
	var decoded sightjack.SessionStartedPayload
	if err := sightjack.UnmarshalEventPayload(event, &decoded); err != nil {
		t.Fatalf("UnmarshalEventPayload: %v", err)
	}

	// then
	if decoded.Project != "my-project" {
		t.Errorf("expected my-project, got %s", decoded.Project)
	}
	if decoded.StrictnessLevel != "fog" {
		t.Errorf("expected fog, got %s", decoded.StrictnessLevel)
	}
}

func TestPayload_ScanCompleted_RoundTrip(t *testing.T) {
	// given
	payload := sightjack.ScanCompletedPayload{
		Clusters: []sightjack.ClusterState{
			{Name: "Auth", Completeness: 0.5, IssueCount: 3},
		},
		Completeness:   0.5,
		ShibitoCount:   2,
		ScanResultPath: "/path/to/scan.json",
		LastScanned:    time.Date(2026, 2, 24, 10, 0, 0, 0, time.UTC),
	}
	event, _ := sightjack.NewEvent(sightjack.EventScanCompleted, "s1", 2, payload)

	// when
	var decoded sightjack.ScanCompletedPayload
	sightjack.UnmarshalEventPayload(event, &decoded)

	// then
	if len(decoded.Clusters) != 1 {
		t.Fatalf("expected 1 cluster, got %d", len(decoded.Clusters))
	}
	if decoded.Clusters[0].Name != "Auth" {
		t.Errorf("expected Auth, got %s", decoded.Clusters[0].Name)
	}
	if decoded.ScanResultPath != "/path/to/scan.json" {
		t.Errorf("expected path, got %s", decoded.ScanResultPath)
	}
}

func TestPayload_WavesGenerated_RoundTrip(t *testing.T) {
	// given
	payload := sightjack.WavesGeneratedPayload{
		Waves: []sightjack.WaveState{
			{ID: "w1", ClusterName: "Auth", Title: "First", Status: "available", ActionCount: 2},
		},
	}
	event, _ := sightjack.NewEvent(sightjack.EventWavesGenerated, "s1", 3, payload)

	// when
	var decoded sightjack.WavesGeneratedPayload
	sightjack.UnmarshalEventPayload(event, &decoded)

	// then
	if len(decoded.Waves) != 1 {
		t.Fatalf("expected 1 wave, got %d", len(decoded.Waves))
	}
	if decoded.Waves[0].ID != "w1" {
		t.Errorf("expected w1, got %s", decoded.Waves[0].ID)
	}
}

func TestPayload_WaveApproved_RoundTrip(t *testing.T) {
	payload := sightjack.WaveIdentityPayload{WaveID: "w1", ClusterName: "Auth"}
	event, _ := sightjack.NewEvent(sightjack.EventWaveApproved, "s1", 4, payload)

	var decoded sightjack.WaveIdentityPayload
	sightjack.UnmarshalEventPayload(event, &decoded)

	if decoded.WaveID != "w1" || decoded.ClusterName != "Auth" {
		t.Errorf("unexpected payload: %+v", decoded)
	}
}

func TestPayload_WaveCompleted_RoundTrip(t *testing.T) {
	payload := sightjack.WaveCompletedPayload{
		WaveID:      "w1",
		ClusterName: "Auth",
		Applied:     3,
		TotalCount:  3,
	}
	event, _ := sightjack.NewEvent(sightjack.EventWaveCompleted, "s1", 5, payload)

	var decoded sightjack.WaveCompletedPayload
	sightjack.UnmarshalEventPayload(event, &decoded)

	if decoded.Applied != 3 {
		t.Errorf("expected 3 applied, got %d", decoded.Applied)
	}
}

func TestPayload_CompletenessUpdated_RoundTrip(t *testing.T) {
	payload := sightjack.CompletenessUpdatedPayload{
		ClusterName:         "Auth",
		ClusterCompleteness: 0.75,
		OverallCompleteness: 0.60,
	}
	event, _ := sightjack.NewEvent(sightjack.EventCompletenessUpdated, "s1", 6, payload)

	var decoded sightjack.CompletenessUpdatedPayload
	sightjack.UnmarshalEventPayload(event, &decoded)

	if decoded.ClusterCompleteness != 0.75 {
		t.Errorf("expected 0.75, got %f", decoded.ClusterCompleteness)
	}
}

func TestPayload_NextGenWavesAdded_RoundTrip(t *testing.T) {
	payload := sightjack.NextGenWavesAddedPayload{
		ClusterName: "Auth",
		Waves: []sightjack.WaveState{
			{ID: "w2", ClusterName: "Auth", Title: "Second", Status: "available"},
		},
	}
	event, _ := sightjack.NewEvent(sightjack.EventNextGenWavesAdded, "s1", 7, payload)

	var decoded sightjack.NextGenWavesAddedPayload
	sightjack.UnmarshalEventPayload(event, &decoded)

	if decoded.ClusterName != "Auth" || len(decoded.Waves) != 1 {
		t.Errorf("unexpected: %+v", decoded)
	}
}

func TestPayload_WaveModified_RoundTrip(t *testing.T) {
	payload := sightjack.WaveModifiedPayload{
		WaveID:      "w1",
		ClusterName: "Auth",
		UpdatedWave: sightjack.WaveState{
			ID: "w1", ClusterName: "Auth", Title: "Modified", Status: "available",
		},
	}
	event, _ := sightjack.NewEvent(sightjack.EventWaveModified, "s1", 8, payload)

	var decoded sightjack.WaveModifiedPayload
	sightjack.UnmarshalEventPayload(event, &decoded)

	if decoded.UpdatedWave.Title != "Modified" {
		t.Errorf("expected Modified, got %s", decoded.UpdatedWave.Title)
	}
}

func TestPayload_ADRGenerated_RoundTrip(t *testing.T) {
	payload := sightjack.ADRGeneratedPayload{ADRID: "0008", Title: "Event Sourcing"}
	event, _ := sightjack.NewEvent(sightjack.EventADRGenerated, "s1", 9, payload)

	var decoded sightjack.ADRGeneratedPayload
	sightjack.UnmarshalEventPayload(event, &decoded)

	if decoded.ADRID != "0008" {
		t.Errorf("expected 0008, got %s", decoded.ADRID)
	}
}

func TestPayload_WavesUnlocked_RoundTrip(t *testing.T) {
	payload := sightjack.WavesUnlockedPayload{
		UnlockedWaveIDs: []string{"Auth:w2", "Auth:w3"},
	}
	event, _ := sightjack.NewEvent(sightjack.EventWavesUnlocked, "s1", 10, payload)

	var decoded sightjack.WavesUnlockedPayload
	sightjack.UnmarshalEventPayload(event, &decoded)

	if len(decoded.UnlockedWaveIDs) != 2 {
		t.Errorf("expected 2 unlocked, got %d", len(decoded.UnlockedWaveIDs))
	}
}

func TestPayload_ReadyLabelsApplied_RoundTrip(t *testing.T) {
	payload := sightjack.ReadyLabelsAppliedPayload{IssueIDs: []string{"ENG-101", "ENG-102"}}
	event, _ := sightjack.NewEvent(sightjack.EventReadyLabelsApplied, "s1", 11, payload)

	var decoded sightjack.ReadyLabelsAppliedPayload
	sightjack.UnmarshalEventPayload(event, &decoded)

	if len(decoded.IssueIDs) != 2 {
		t.Errorf("expected 2 issues, got %d", len(decoded.IssueIDs))
	}
}

func TestPayload_WaveApplied_RoundTrip(t *testing.T) {
	payload := sightjack.WaveAppliedPayload{
		WaveID:      "w1",
		ClusterName: "Auth",
		Applied:     2,
		TotalCount:  3,
		Errors:      []string{"action 3 failed"},
	}
	event, _ := sightjack.NewEvent(sightjack.EventWaveApplied, "s1", 12, payload)

	var decoded sightjack.WaveAppliedPayload
	sightjack.UnmarshalEventPayload(event, &decoded)

	if decoded.Applied != 2 || len(decoded.Errors) != 1 {
		t.Errorf("unexpected: %+v", decoded)
	}
}
