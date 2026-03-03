package domain_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
)

func TestConfigPath(t *testing.T) {
	// when
	path := domain.ConfigPath("/project")

	// then
	expected := filepath.Join("/project", ".siren", "config.yaml")
	if path != expected {
		t.Errorf("expected %s, got %s", expected, path)
	}
}

func TestSessionState_ADRCount_Positive(t *testing.T) {
	// given
	state := domain.SessionState{
		Version:  "0.4",
		ADRCount: 3,
	}

	// when
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded domain.SessionState
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
	state := domain.SessionState{
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
	state := domain.SessionState{Version: "0.5", ScanResultPath: ""}

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

func TestRelativeScanResultPath(t *testing.T) {
	// given
	baseDir := "/project"
	absPath := "/project/.siren/.run/session-1/scan_result.json"

	// when
	rel := domain.RelativeScanResultPath(baseDir, absPath)

	// then
	want := filepath.Join(".siren", ".run", "session-1", "scan_result.json")
	if rel != want {
		t.Errorf("RelativeScanResultPath = %q, want %q", rel, want)
	}
}

func TestRelativeScanResultPath_AlreadyRelative(t *testing.T) {
	// given — already relative path
	baseDir := "/project"
	relPath := ".siren/.run/session-1/scan_result.json"

	// when
	result := domain.RelativeScanResultPath(baseDir, relPath)

	// then — returns as-is since it's already relative
	if result != relPath {
		t.Errorf("RelativeScanResultPath = %q, want %q (unchanged)", result, relPath)
	}
}

func TestResolveScanResultPath_Relative(t *testing.T) {
	// given
	baseDir := "/project"
	storedPath := filepath.Join(".siren", ".run", "session-1", "scan_result.json")

	// when
	abs := domain.ResolveScanResultPath(baseDir, storedPath)

	// then
	want := filepath.Join("/project", ".siren", ".run", "session-1", "scan_result.json")
	if abs != want {
		t.Errorf("ResolveScanResultPath = %q, want %q", abs, want)
	}
}

func TestResolveScanResultPath_Absolute(t *testing.T) {
	// given — backwards compatibility: absolute path stored in old events
	baseDir := "/project"
	storedPath := "/old-project/.siren/.run/session-1/scan_result.json"

	// when
	abs := domain.ResolveScanResultPath(baseDir, storedPath)

	// then — absolute path returned as-is
	if abs != storedPath {
		t.Errorf("ResolveScanResultPath = %q, want %q (unchanged)", abs, storedPath)
	}
}

func TestResolveScanResultPath_Empty(t *testing.T) {
	// given
	abs := domain.ResolveScanResultPath("/project", "")

	// then
	if abs != "" {
		t.Errorf("ResolveScanResultPath empty = %q, want empty", abs)
	}
}
