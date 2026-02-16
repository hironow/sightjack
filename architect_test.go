package sightjack

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestParseArchitectResult(t *testing.T) {
	// given: a valid architect response JSON file
	dir := t.TempDir()
	path := filepath.Join(dir, "architect_auth_auth-w1.json")
	data := ArchitectResponse{
		Analysis: "Splitting is unnecessary.",
		ModifiedWave: &Wave{
			ID:          "auth-w1",
			ClusterName: "Auth",
			Title:       "Dependency Ordering",
			Actions: []WaveAction{
				{Type: "add_dependency", IssueID: "ENG-101", Description: "Auth before token"},
				{Type: "add_dod", IssueID: "ENG-101", Description: "Middleware interface"},
			},
			Delta:  WaveDelta{Before: 0.25, After: 0.42},
			Status: "available",
		},
		Reasoning: "Project scale favors fewer issues.",
	}
	raw, _ := json.MarshalIndent(data, "", "  ")
	os.WriteFile(path, raw, 0644)

	// when
	result, err := ParseArchitectResult(path)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Analysis != "Splitting is unnecessary." {
		t.Errorf("unexpected analysis: %s", result.Analysis)
	}
	if result.ModifiedWave == nil {
		t.Fatal("expected non-nil modified_wave")
	}
	if len(result.ModifiedWave.Actions) != 2 {
		t.Errorf("expected 2 actions, got %d", len(result.ModifiedWave.Actions))
	}
}

func TestParseArchitectResult_NilWave(t *testing.T) {
	// given
	dir := t.TempDir()
	path := filepath.Join(dir, "architect.json")
	os.WriteFile(path, []byte(`{"analysis":"No changes.","modified_wave":null,"reasoning":"OK"}`), 0644)

	// when
	result, err := ParseArchitectResult(path)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ModifiedWave != nil {
		t.Error("expected nil modified_wave")
	}
}

func TestParseArchitectResult_FileNotFound(t *testing.T) {
	// when
	_, err := ParseArchitectResult("/nonexistent/path.json")

	// then
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestArchitectDiscussFileName(t *testing.T) {
	wave := Wave{ID: "auth-w1", ClusterName: "Auth"}
	name := architectDiscussFileName(wave)
	if name != "architect_auth_auth-w1.json" {
		t.Errorf("expected architect_auth_auth-w1.json, got %s", name)
	}
}

func TestArchitectDiscussFileName_SpecialChars(t *testing.T) {
	wave := Wave{ID: "w-1", ClusterName: "UI/Frontend"}
	name := architectDiscussFileName(wave)
	if name != "architect_ui_frontend_w-1.json" {
		t.Errorf("expected architect_ui_frontend_w-1.json, got %s", name)
	}
}
