package sightjack

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		t.Fatalf("marshal test data: %v", err)
	}
	if err := os.WriteFile(path, raw, 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

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
	if err := os.WriteFile(path, []byte(`{"analysis":"No changes.","modified_wave":null,"reasoning":"OK"}`), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

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

func TestRunArchitectDiscuss_DryRun(t *testing.T) {
	// given
	scanDir := t.TempDir()
	cfg := &Config{
		Lang:   "en",
		Claude: ClaudeConfig{Command: "claude", TimeoutSec: 60},
	}
	wave := Wave{
		ID:          "auth-w1",
		ClusterName: "Auth",
		Title:       "Dependency Ordering",
		Actions:     []WaveAction{{Type: "add_dependency", IssueID: "ENG-101", Description: "test"}},
	}

	// when
	err := RunArchitectDiscussDryRun(cfg, scanDir, wave, "test topic")

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	promptFile := filepath.Join(scanDir, "architect_auth_auth-w1_prompt.md")
	if _, err := os.Stat(promptFile); os.IsNotExist(err) {
		t.Error("expected architect prompt file to be generated")
	}
}

func TestParseArchitectResult_MalformedJSON(t *testing.T) {
	// given: file exists but contains truncated/invalid JSON (realistic: Claude output cut off)
	dir := t.TempDir()
	path := filepath.Join(dir, "architect.json")
	if err := os.WriteFile(path, []byte(`{"analysis": "truncated`), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	// when
	_, err := ParseArchitectResult(path)

	// then
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "parse architect result") {
		t.Errorf("expected 'parse architect result' in error, got: %v", err)
	}
}

func TestParseArchitectResult_ModifiedWaveNilActions(t *testing.T) {
	// given: Claude returns modified_wave but omits the "actions" field entirely
	dir := t.TempDir()
	path := filepath.Join(dir, "architect.json")
	data := []byte(`{
		"analysis": "Modified wave proposed",
		"modified_wave": {
			"id": "auth-w1",
			"cluster_name": "Auth",
			"title": "Modified",
			"delta": {"before": 0.25, "after": 0.40},
			"status": "available"
		},
		"reasoning": "Simplified"
	}`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	// when
	result, err := ParseArchitectResult(path)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ModifiedWave == nil {
		t.Fatal("expected non-nil modified_wave")
	}
	if result.ModifiedWave.Actions != nil {
		t.Errorf("expected nil Actions when field omitted, got %v", result.ModifiedWave.Actions)
	}
	// Verify ranging over nil Actions is safe (this is Go's guarantee but documents the contract)
	for range result.ModifiedWave.Actions {
		t.Error("should not iterate over nil actions")
	}
}

func TestRunArchitectDiscussDryRun_NilActions(t *testing.T) {
	// given: wave with nil Actions — json.Marshal produces "null" not "[]"
	scanDir := t.TempDir()
	cfg := &Config{
		Lang:   "en",
		Claude: ClaudeConfig{Command: "claude", TimeoutSec: 60},
	}
	wave := Wave{
		ID:          "auth-w1",
		ClusterName: "Auth",
		Title:       "Empty Wave",
		Actions:     nil, // explicitly nil
	}

	// when
	err := RunArchitectDiscussDryRun(cfg, scanDir, wave, "test topic")

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	promptFile := filepath.Join(scanDir, "architect_auth_auth-w1_prompt.md")
	content, readErr := os.ReadFile(promptFile)
	if readErr != nil {
		t.Fatalf("failed to read prompt file: %v", readErr)
	}
	// json.Marshal(nil) for []WaveAction produces "null"
	if !strings.Contains(string(content), "null") {
		t.Error("expected 'null' in prompt for nil actions")
	}
}

func TestRunArchitectDiscuss_RemovesStaleOutputBeforeRun(t *testing.T) {
	// given: a pre-existing stale output file from a previous discuss round
	scanDir := t.TempDir()
	wave := Wave{ID: "auth-w1", ClusterName: "Auth", Title: "Test"}
	outputFile := filepath.Join(scanDir, architectDiscussFileName(wave))
	if err := os.WriteFile(outputFile, []byte(`{"analysis":"stale","modified_wave":null,"reasoning":"old"}`), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	// when: verify the stale file exists before the function runs
	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		t.Fatal("precondition: stale file should exist")
	}

	// then: clearArchitectOutput removes it
	clearArchitectOutput(scanDir, wave)

	if _, err := os.Stat(outputFile); !os.IsNotExist(err) {
		t.Error("expected stale output file to be removed")
	}
}
