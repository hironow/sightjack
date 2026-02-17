package sightjack

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestState_WriteAndRead_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	state := SessionState{
		Version:      "0.1",
		SessionID:    "test-session-123",
		Project:      "Test Project",
		LastScanned:  time.Date(2026, 2, 16, 10, 0, 0, 0, time.UTC),
		Completeness: 0.32,
		Clusters: []ClusterState{
			{Name: "Auth", Completeness: 0.25, IssueCount: 5},
			{Name: "API", Completeness: 0.40, IssueCount: 8},
		},
	}

	err := WriteState(dir, &state)
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	loaded, err := ReadState(dir)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	if loaded.SessionID != "test-session-123" {
		t.Errorf("expected session ID test-session-123, got %s", loaded.SessionID)
	}
	if loaded.Completeness != 0.32 {
		t.Errorf("expected 0.32, got %f", loaded.Completeness)
	}
	if len(loaded.Clusters) != 2 {
		t.Fatalf("expected 2 clusters, got %d", len(loaded.Clusters))
	}
	if loaded.Clusters[0].Name != "Auth" {
		t.Errorf("expected Auth, got %s", loaded.Clusters[0].Name)
	}
}

func TestState_ReadMissing_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	_, err := ReadState(dir)
	if err == nil {
		t.Error("expected error for missing state file")
	}
}

func TestState_WriteAndRead_WithWaves(t *testing.T) {
	// given
	dir := t.TempDir()
	state := &SessionState{
		Version:      "0.2",
		SessionID:    "test-session",
		Project:      "TestProject",
		LastScanned:  time.Now().Truncate(time.Second),
		Completeness: 0.35,
		Clusters: []ClusterState{
			{Name: "Auth", Completeness: 0.25, IssueCount: 4},
		},
		Waves: []WaveState{
			{ID: "auth-w1", ClusterName: "Auth", Title: "Deps", Status: "completed", ActionCount: 3},
			{ID: "auth-w2", ClusterName: "Auth", Title: "DoD", Status: "available", Prerequisites: []string{"auth-w1"}, ActionCount: 5},
		},
	}

	// when
	if err := WriteState(dir, state); err != nil {
		t.Fatalf("write: %v", err)
	}
	loaded, err := ReadState(dir)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	// then
	if len(loaded.Waves) != 2 {
		t.Fatalf("expected 2 waves, got %d", len(loaded.Waves))
	}
	if loaded.Waves[0].ID != "auth-w1" {
		t.Errorf("expected auth-w1, got %s", loaded.Waves[0].ID)
	}
	if loaded.Waves[1].Status != "available" {
		t.Errorf("expected available, got %s", loaded.Waves[1].Status)
	}
	if loaded.Waves[1].Prerequisites[0] != "auth-w1" {
		t.Errorf("expected prerequisite auth-w1")
	}
}

func TestStatePath(t *testing.T) {
	path := StatePath("/project")
	expected := filepath.Join("/project", ".siren", "state.json")
	if path != expected {
		t.Errorf("expected %s, got %s", expected, path)
	}
}

func TestSessionState_ADRCount_Positive(t *testing.T) {
	// given
	state := SessionState{
		Version:  "0.4",
		ADRCount: 3,
	}

	// when
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded SessionState
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// then
	if decoded.ADRCount != 3 {
		t.Errorf("expected ADRCount 3, got %d", decoded.ADRCount)
	}
}

func TestSessionState_ADRCount_ZeroOmitted(t *testing.T) {
	// given: ADRCount = 0 should be omitted from JSON (omitempty)
	state := SessionState{
		Version:  "0.4",
		ADRCount: 0,
	}

	// when
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// then: "adr_count" should not appear in JSON
	raw := string(data)
	if json.Valid(data) && strings.Contains(raw, "adr_count") {
		t.Errorf("expected adr_count to be omitted when 0, got: %s", raw)
	}
}

func TestSessionState_ADRCount_WriteAndRead(t *testing.T) {
	// given
	dir := t.TempDir()
	state := &SessionState{
		Version:      "0.4",
		SessionID:    "test-adr-count",
		Project:      "TestProject",
		LastScanned:  time.Now().Truncate(time.Second),
		Completeness: 0.50,
		Clusters:     []ClusterState{{Name: "Auth", Completeness: 0.50, IssueCount: 3}},
		ADRCount:     5,
	}

	// when
	if err := WriteState(dir, state); err != nil {
		t.Fatalf("write: %v", err)
	}
	loaded, err := ReadState(dir)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	// then
	if loaded.ADRCount != 5 {
		t.Errorf("expected ADRCount 5, got %d", loaded.ADRCount)
	}
}
