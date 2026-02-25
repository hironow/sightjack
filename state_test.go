package sightjack_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/sightjack"
)

func TestConfigPath(t *testing.T) {
	// when
	path := sightjack.ConfigPath("/project")

	// then
	expected := filepath.Join("/project", ".siren", "config.yaml")
	if path != expected {
		t.Errorf("expected %s, got %s", expected, path)
	}
}

func TestSessionState_ADRCount_Positive(t *testing.T) {
	// given
	state := sightjack.SessionState{
		Version:  "0.4",
		ADRCount: 3,
	}

	// when
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded sightjack.SessionState
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
	state := sightjack.SessionState{
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

func TestSessionState_ScanResultPath_OmittedWhenEmpty(t *testing.T) {
	// given
	state := sightjack.SessionState{Version: "0.5", ScanResultPath: ""}

	// when
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// then
	if strings.Contains(string(data), "scan_result_path") {
		t.Error("expected scan_result_path to be omitted when empty")
	}
}
