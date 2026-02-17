package sightjack

import (
	"strings"
	"testing"
)

func TestRenderClassifyPrompt(t *testing.T) {
	// given
	data := ClassifyPromptData{
		TeamFilter:    "MY-TEAM",
		ProjectFilter: "My Project",
		OutputPath:    "/tmp/classify.json",
	}

	// when
	result, err := RenderClassifyPrompt("ja", data)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "MY-TEAM") {
		t.Error("expected team filter in prompt")
	}
	if !strings.Contains(result, "/tmp/classify.json") {
		t.Error("expected output path in prompt")
	}
}

func TestRenderDeepScanPrompt(t *testing.T) {
	// given
	data := DeepScanPromptData{
		ClusterName: "Auth",
		IssueIDs:    "ID1, ID2, ID3",
		OutputPath:  "/tmp/cluster_auth.json",
	}

	// when
	result, err := RenderDeepScanPrompt("ja", data)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Auth") {
		t.Error("expected cluster name in prompt")
	}
	if !strings.Contains(result, "/tmp/cluster_auth.json") {
		t.Error("expected output path in prompt")
	}
}

func TestRenderWaveGeneratePrompt(t *testing.T) {
	// given
	data := WaveGeneratePromptData{
		ClusterName:  "Auth",
		Completeness: "25",
		Issues:       `[{"id":"ENG-101","title":"Login","completeness":0.3,"gaps":["No DoD"]}]`,
		Observations: "Cross-cluster dependency detected",
		OutputPath:   "/tmp/wave_auth.json",
	}

	// when
	result, err := RenderWaveGeneratePrompt("ja", data)

	// then
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !strings.Contains(result, "Auth") {
		t.Error("expected cluster name in output")
	}
	if !strings.Contains(result, "/tmp/wave_auth.json") {
		t.Error("expected output path in output")
	}
}

func TestRenderWaveGeneratePrompt_English(t *testing.T) {
	// given
	data := WaveGeneratePromptData{
		ClusterName:  "Auth",
		Completeness: "25",
		Issues:       `[{"id":"ENG-101","title":"Login"}]`,
		Observations: "Dependency detected",
		OutputPath:   "/tmp/wave_auth.json",
	}

	// when
	result, err := RenderWaveGeneratePrompt("en", data)

	// then
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !strings.Contains(result, "Scanner Agent") {
		t.Error("expected Scanner Agent in English prompt")
	}
	if !strings.Contains(result, "Auth") {
		t.Error("expected cluster name in output")
	}
}

func TestRenderWaveApplyPrompt(t *testing.T) {
	// given
	data := WaveApplyPromptData{
		WaveID:      "auth-w1",
		ClusterName: "Auth",
		Title:       "Dependency Ordering",
		Actions:     `[{"type":"add_dependency","issue_id":"ENG-101","description":"Auth before token","detail":"ENG-101 -> ENG-102"}]`,
		OutputPath:  "/tmp/apply_auth-w1.json",
	}

	// when
	result, err := RenderWaveApplyPrompt("ja", data)

	// then
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !strings.Contains(result, "auth-w1") {
		t.Error("expected wave ID in output")
	}
	if !strings.Contains(result, "/tmp/apply_auth-w1.json") {
		t.Error("expected output path in output")
	}
}

func TestRenderWaveApplyPrompt_English(t *testing.T) {
	// given
	data := WaveApplyPromptData{
		WaveID:      "auth-w1",
		ClusterName: "Auth",
		Title:       "Dependency Ordering",
		Actions:     `[{"type":"add_dependency","issue_id":"ENG-101"}]`,
		OutputPath:  "/tmp/apply_auth-w1.json",
	}

	// when
	result, err := RenderWaveApplyPrompt("en", data)

	// then
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !strings.Contains(result, "Scanner Agent") {
		t.Error("expected Scanner Agent in English prompt")
	}
	if !strings.Contains(result, "auth-w1") {
		t.Error("expected wave ID in output")
	}
}

func TestRenderArchitectDiscussPrompt(t *testing.T) {
	// given
	data := ArchitectDiscussPromptData{
		ClusterName: "Auth",
		WaveTitle:   "Dependency Ordering",
		WaveActions: `[{"type":"add_dependency","issue_id":"ENG-101","description":"Auth before token"}]`,
		Topic:       "Should we split ENG-101?",
		OutputPath:  "/tmp/architect_auth_auth-w1.json",
	}

	// when
	result, err := RenderArchitectDiscussPrompt("en", data)

	// then
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !strings.Contains(result, "Auth") {
		t.Error("expected cluster name in output")
	}
	if !strings.Contains(result, "Should we split ENG-101?") {
		t.Error("expected topic in output")
	}
	if !strings.Contains(result, "/tmp/architect_auth_auth-w1.json") {
		t.Error("expected output path in output")
	}
}

func TestRenderArchitectDiscussPrompt_Japanese(t *testing.T) {
	// given
	data := ArchitectDiscussPromptData{
		ClusterName: "Auth",
		WaveTitle:   "Dependency Ordering",
		WaveActions: `[{"type":"add_dependency","issue_id":"ENG-101"}]`,
		Topic:       "ENG-101を分割すべき？",
		OutputPath:  "/tmp/out.json",
	}

	// when
	result, err := RenderArchitectDiscussPrompt("ja", data)

	// then
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !strings.Contains(result, "Architect Agent") {
		t.Error("expected Architect Agent in Japanese prompt")
	}
	if !strings.Contains(result, "Auth") {
		t.Error("expected cluster name in output")
	}
}

func TestRenderArchitectDiscussPrompt_UnsupportedLang(t *testing.T) {
	// given: unsupported language code
	data := ArchitectDiscussPromptData{
		ClusterName: "Auth",
		WaveTitle:   "Test",
		WaveActions: "[]",
		Topic:       "test",
		OutputPath:  "/tmp/out.json",
	}

	// when
	_, err := RenderArchitectDiscussPrompt("fr", data)

	// then
	if err == nil {
		t.Fatal("expected error for unsupported language")
	}
}

func TestRenderArchitectDiscussPrompt_SpecialCharsInTopic(t *testing.T) {
	// given: topic containing Go template delimiters — should pass through as data, not template syntax
	data := ArchitectDiscussPromptData{
		ClusterName: "Auth",
		WaveTitle:   "Test",
		WaveActions: "[]",
		Topic:       "What if we use {{interface}} here?",
		OutputPath:  "/tmp/out.json",
	}

	// when
	result, err := RenderArchitectDiscussPrompt("en", data)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "{{interface}}") {
		t.Error("expected template delimiters in topic to pass through as literal text")
	}
}

func TestRenderArchitectDiscussPrompt_EmptyWaveActions(t *testing.T) {
	// given: empty string for WaveActions (not "[]" or "null")
	data := ArchitectDiscussPromptData{
		ClusterName: "Auth",
		WaveTitle:   "Test",
		WaveActions: "",
		Topic:       "test topic",
		OutputPath:  "/tmp/out.json",
	}

	// when
	result, err := RenderArchitectDiscussPrompt("en", data)

	// then: renders successfully with empty section
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Auth") {
		t.Error("expected cluster name in output")
	}
}

func TestRenderClassifyPrompt_English(t *testing.T) {
	// given
	data := ClassifyPromptData{
		TeamFilter:    "TEST",
		ProjectFilter: "Test",
		OutputPath:    "/tmp/out.json",
	}

	// when
	result, err := RenderClassifyPrompt("en", data)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Scanner Agent") {
		t.Error("expected Scanner Agent in English prompt")
	}
}
