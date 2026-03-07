package platform_test

import (
	"strings"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/platform"
)

func TestRenderClassifyPrompt(t *testing.T) {
	// given
	data := domain.ClassifyPromptData{
		TeamFilter:    "MY-TEAM",
		ProjectFilter: "My Project",
		OutputPath:    "/tmp/classify.json",
	}

	// when
	result, err := platform.RenderClassifyPrompt("ja", data)

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
	data := domain.DeepScanPromptData{
		ClusterName: "Auth",
		IssueIDs:    "ID1, ID2, ID3",
		OutputPath:  "/tmp/cluster_auth.json",
	}

	// when
	result, err := platform.RenderDeepScanPrompt("ja", data)

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
	data := domain.WaveGeneratePromptData{
		ClusterName:  "Auth",
		Completeness: "25",
		Issues:       `[{"id":"ENG-101","title":"Login","completeness":0.3,"gaps":["No DoD"]}]`,
		Observations: "Cross-cluster dependency detected",
		OutputPath:   "/tmp/wave_auth.json",
	}

	// when
	result, err := platform.RenderWaveGeneratePrompt("ja", data)

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
	data := domain.WaveGeneratePromptData{
		ClusterName:  "Auth",
		Completeness: "25",
		Issues:       `[{"id":"ENG-101","title":"Login"}]`,
		Observations: "Dependency detected",
		OutputPath:   "/tmp/wave_auth.json",
	}

	// when
	result, err := platform.RenderWaveGeneratePrompt("en", data)

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
	data := domain.WaveApplyPromptData{
		WaveID:      "auth-w1",
		ClusterName: "Auth",
		Title:       "Dependency Ordering",
		Actions:     `[{"type":"add_dependency","issue_id":"ENG-101","description":"Auth before token","detail":"ENG-101 -> ENG-102"}]`,
		OutputPath:  "/tmp/apply_auth-w1.json",
	}

	// when
	result, err := platform.RenderWaveApplyPrompt("ja", data)

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
	data := domain.WaveApplyPromptData{
		WaveID:      "auth-w1",
		ClusterName: "Auth",
		Title:       "Dependency Ordering",
		Actions:     `[{"type":"add_dependency","issue_id":"ENG-101"}]`,
		OutputPath:  "/tmp/apply_auth-w1.json",
	}

	// when
	result, err := platform.RenderWaveApplyPrompt("en", data)

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
	data := domain.ArchitectDiscussPromptData{
		ClusterName: "Auth",
		WaveTitle:   "Dependency Ordering",
		WaveActions: `[{"type":"add_dependency","issue_id":"ENG-101","description":"Auth before token"}]`,
		Topic:       "Should we split ENG-101?",
		OutputPath:  "/tmp/architect_auth_auth-w1.json",
	}

	// when
	result, err := platform.RenderArchitectDiscussPrompt("en", data)

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
	data := domain.ArchitectDiscussPromptData{
		ClusterName: "Auth",
		WaveTitle:   "Dependency Ordering",
		WaveActions: `[{"type":"add_dependency","issue_id":"ENG-101"}]`,
		Topic:       "ENG-101を分割すべき？",
		OutputPath:  "/tmp/out.json",
	}

	// when
	result, err := platform.RenderArchitectDiscussPrompt("ja", data)

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
	data := domain.ArchitectDiscussPromptData{
		ClusterName: "Auth",
		WaveTitle:   "Test",
		WaveActions: "[]",
		Topic:       "test",
		OutputPath:  "/tmp/out.json",
	}

	// when
	_, err := platform.RenderArchitectDiscussPrompt("fr", data)

	// then
	if err == nil {
		t.Fatal("expected error for unsupported language")
	}
}

func TestRenderArchitectDiscussPrompt_SpecialCharsInTopic(t *testing.T) {
	// given: topic containing Go template delimiters — should pass through as data, not template syntax
	data := domain.ArchitectDiscussPromptData{
		ClusterName: "Auth",
		WaveTitle:   "Test",
		WaveActions: "[]",
		Topic:       "What if we use {{interface}} here?",
		OutputPath:  "/tmp/out.json",
	}

	// when
	result, err := platform.RenderArchitectDiscussPrompt("en", data)

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
	data := domain.ArchitectDiscussPromptData{
		ClusterName: "Auth",
		WaveTitle:   "Test",
		WaveActions: "",
		Topic:       "test topic",
		OutputPath:  "/tmp/out.json",
	}

	// when
	result, err := platform.RenderArchitectDiscussPrompt("en", data)

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
	data := domain.ClassifyPromptData{
		TeamFilter:    "TEST",
		ProjectFilter: "Test",
		OutputPath:    "/tmp/out.json",
	}

	// when
	result, err := platform.RenderClassifyPrompt("en", data)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Scanner Agent") {
		t.Error("expected Scanner Agent in English prompt")
	}
}

func TestRenderScribeADRPrompt_English(t *testing.T) {
	// given
	data := domain.ScribeADRPromptData{
		ClusterName: "Auth",
		WaveTitle:   "Dependency Ordering",
		WaveActions: `[{"type":"add_dependency","issue_id":"ENG-101"}]`,
		Analysis:    "Splitting recommended.",
		Reasoning:   "Project scale favors smaller batches.",
		ADRNumber:   "0003",
		OutputPath:  "/tmp/scribe_auth_auth-w1.json",
	}

	// when
	result, err := platform.RenderScribeADRPrompt("en", data)

	// then
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !strings.Contains(result, "Scribe Agent") {
		t.Error("expected Scribe Agent in English prompt")
	}
	if !strings.Contains(result, "Auth") {
		t.Error("expected cluster name in prompt")
	}
	if !strings.Contains(result, "0003") {
		t.Error("expected ADR number in prompt")
	}
	if !strings.Contains(result, "/tmp/scribe_auth_auth-w1.json") {
		t.Error("expected output path in prompt")
	}
}

func TestRenderScribeADRPrompt_Japanese(t *testing.T) {
	// given
	data := domain.ScribeADRPromptData{
		ClusterName: "Auth",
		WaveTitle:   "Dependency Ordering",
		WaveActions: `[{"type":"add_dependency","issue_id":"ENG-101"}]`,
		Analysis:    "分割を推奨。",
		Reasoning:   "プロジェクト規模に適している。",
		ADRNumber:   "0001",
		OutputPath:  "/tmp/scribe.json",
	}

	// when
	result, err := platform.RenderScribeADRPrompt("ja", data)

	// then
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !strings.Contains(result, "Scribe Agent") {
		t.Error("expected Scribe Agent in Japanese prompt")
	}
	if !strings.Contains(result, "Auth") {
		t.Error("expected cluster name in prompt")
	}
	if !strings.Contains(result, "0001") {
		t.Error("expected ADR number in prompt")
	}
}

func TestRenderClassifyPrompt_ContainsStrictnessLevel(t *testing.T) {
	// given
	data := domain.ClassifyPromptData{
		TeamFilter:      "TEST",
		ProjectFilter:   "Test",
		OutputPath:      "/tmp/out.json",
		StrictnessLevel: "alert",
	}

	// when
	result, err := platform.RenderClassifyPrompt("en", data)

	// then
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !strings.Contains(result, "alert") {
		t.Error("expected strictness level 'alert' in prompt")
	}
}

func TestRenderWaveGeneratePrompt_ContainsStrictnessLevel(t *testing.T) {
	// given
	data := domain.WaveGeneratePromptData{
		ClusterName:     "Auth",
		Completeness:    "25",
		Issues:          "[]",
		Observations:    "none",
		OutputPath:      "/tmp/out.json",
		StrictnessLevel: "lockdown",
	}

	// when
	result, err := platform.RenderWaveGeneratePrompt("en", data)

	// then
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !strings.Contains(result, "lockdown") {
		t.Error("expected strictness level 'lockdown' in prompt")
	}
}

func TestRenderScribeADRPrompt_ContainsStrictnessLevel(t *testing.T) {
	// given
	data := domain.ScribeADRPromptData{
		ClusterName:     "Auth",
		WaveTitle:       "Test",
		WaveActions:     "[]",
		Analysis:        "test",
		Reasoning:       "test",
		ADRNumber:       "0001",
		OutputPath:      "/tmp/out.json",
		StrictnessLevel: "fog",
	}

	// when
	result, err := platform.RenderScribeADRPrompt("en", data)

	// then
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !strings.Contains(result, "fog") {
		t.Error("expected strictness level 'fog' in prompt")
	}
}

func TestRenderClassifyPrompt_ContainsShibitoSection(t *testing.T) {
	// given
	data := domain.ClassifyPromptData{
		TeamFilter:      "TEST",
		ProjectFilter:   "Test",
		OutputPath:      "/tmp/out.json",
		StrictnessLevel: "alert",
	}

	// when
	result, err := platform.RenderClassifyPrompt("en", data)

	// then
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !strings.Contains(result, "shibito_warnings") {
		t.Error("expected shibito_warnings in scanner prompt output schema")
	}
}

func TestRenderClassifyPrompt_Japanese_ContainsShibitoSection(t *testing.T) {
	// given
	data := domain.ClassifyPromptData{
		TeamFilter:      "TEST",
		ProjectFilter:   "Test",
		OutputPath:      "/tmp/out.json",
		StrictnessLevel: "fog",
	}

	// when
	result, err := platform.RenderClassifyPrompt("ja", data)

	// then
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !strings.Contains(result, "shibito_warnings") {
		t.Error("expected shibito_warnings in Japanese scanner prompt")
	}
}

func TestRenderScribeADRPrompt_ContainsExistingADRs(t *testing.T) {
	// given
	data := domain.ScribeADRPromptData{
		ClusterName:     "Auth",
		WaveTitle:       "Test",
		WaveActions:     "[]",
		Analysis:        "test",
		Reasoning:       "test",
		ADRNumber:       "0003",
		OutputPath:      "/tmp/out.json",
		StrictnessLevel: "alert",
		ExistingADRs: []domain.ExistingADR{
			{Filename: "0001-auth.md", Content: "# 0001. Auth\nAccepted"},
			{Filename: "0002-api.md", Content: "# 0002. API\nAccepted"},
		},
	}

	// when
	result, err := platform.RenderScribeADRPrompt("en", data)

	// then
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !strings.Contains(result, "0001-auth.md") {
		t.Error("expected existing ADR filename in prompt")
	}
	if !strings.Contains(result, "# 0001. Auth") {
		t.Error("expected existing ADR content in prompt")
	}
	if !strings.Contains(result, "conflicts") {
		t.Error("expected conflicts field in output schema")
	}
}

func TestRenderScribeADRPrompt_NoExistingADRs(t *testing.T) {
	// given
	data := domain.ScribeADRPromptData{
		ClusterName:     "Auth",
		WaveTitle:       "Test",
		WaveActions:     "[]",
		Analysis:        "test",
		Reasoning:       "test",
		ADRNumber:       "0001",
		OutputPath:      "/tmp/out.json",
		StrictnessLevel: "fog",
	}

	// when
	result, err := platform.RenderScribeADRPrompt("en", data)

	// then
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !strings.Contains(result, "Scribe Agent") {
		t.Error("expected Scribe Agent in prompt")
	}
}

func TestRenderScribeADRPrompt_UnsupportedLang(t *testing.T) {
	// given
	data := domain.ScribeADRPromptData{
		ClusterName: "Auth",
		ADRNumber:   "0001",
		OutputPath:  "/tmp/out.json",
	}

	// when
	_, err := platform.RenderScribeADRPrompt("fr", data)

	// then
	if err == nil {
		t.Fatal("expected error for unsupported language")
	}
}

func TestRenderNextGenPrompt_English(t *testing.T) {
	// given
	data := domain.NextGenPromptData{
		ClusterName:     "Auth",
		Completeness:    "65",
		Issues:          `[{"id":"ENG-101"}]`,
		CompletedWaves:  `[{"id":"auth-w1","title":"Initial setup"}]`,
		ExistingADRs:    []domain.ExistingADR{{Filename: "0001-jwt.md", Content: "# JWT decision"}},
		RejectedActions: `[{"type":"add_dod","issue_id":"ENG-102","description":"Rejected action"}]`,
		OutputPath:      "/tmp/nextgen.json",
		StrictnessLevel: "alert",
	}

	// when
	result, err := platform.RenderNextGenPrompt("en", data)

	// then
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	for _, want := range []string{"Auth", "65", "ENG-101", "auth-w1", "0001-jwt.md", "JWT decision", "Rejected action", "/tmp/nextgen.json", "alert"} {
		if !strings.Contains(result, want) {
			t.Errorf("missing %q in output", want)
		}
	}
}

func TestRenderNextGenPrompt_Japanese(t *testing.T) {
	// given
	data := domain.NextGenPromptData{
		ClusterName:     "API",
		Completeness:    "50",
		Issues:          `[]`,
		CompletedWaves:  `[]`,
		OutputPath:      "/tmp/out.json",
		StrictnessLevel: "fog",
	}

	// when
	result, err := platform.RenderNextGenPrompt("ja", data)

	// then
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(result, "API") {
		t.Errorf("missing cluster name in output")
	}
}

func TestRenderNextGenPrompt_NoADRs(t *testing.T) {
	// given
	data := domain.NextGenPromptData{
		ClusterName:     "DB",
		Completeness:    "40",
		Issues:          `[]`,
		CompletedWaves:  `[]`,
		OutputPath:      "/tmp/out.json",
		StrictnessLevel: "fog",
	}

	// when
	result, err := platform.RenderNextGenPrompt("en", data)

	// then
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(result, "Existing ADRs") {
		t.Errorf("ADR section should be omitted when no ADRs exist")
	}
}

func TestRenderNextGenPrompt_NoRejectedActions(t *testing.T) {
	// given
	data := domain.NextGenPromptData{
		ClusterName:     "DB",
		Completeness:    "40",
		Issues:          `[]`,
		CompletedWaves:  `[]`,
		OutputPath:      "/tmp/out.json",
		StrictnessLevel: "fog",
	}

	// when
	result, err := platform.RenderNextGenPrompt("en", data)

	// then
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(result, "Rejected Actions") {
		t.Errorf("Rejected section should be omitted when empty")
	}
}

func TestRenderWaveGeneratePromptWithDoD(t *testing.T) {
	// given
	data := domain.WaveGeneratePromptData{
		ClusterName:     "auth",
		Completeness:    "50",
		Issues:          "[]",
		Observations:    "none",
		OutputPath:      "/tmp/out.json",
		StrictnessLevel: "fog",
		DoDSection:      "Must:\n- Unit tests\nShould:\n- Integration tests\n",
	}

	// when / then
	for _, lang := range []string{"en", "ja"} {
		result, err := platform.RenderWaveGeneratePrompt(lang, data)
		if err != nil {
			t.Fatalf("RenderWaveGeneratePrompt(%s): %v", lang, err)
		}
		if !strings.Contains(result, "Unit tests") {
			t.Errorf("lang=%s: expected DoD section in output", lang)
		}
	}
}

func TestRenderWaveGeneratePromptWithoutDoD(t *testing.T) {
	// given
	data := domain.WaveGeneratePromptData{
		ClusterName:     "auth",
		Completeness:    "50",
		Issues:          "[]",
		Observations:    "none",
		OutputPath:      "/tmp/out.json",
		StrictnessLevel: "fog",
		DoDSection:      "",
	}

	// when / then
	for _, lang := range []string{"en", "ja"} {
		result, err := platform.RenderWaveGeneratePrompt(lang, data)
		if err != nil {
			t.Fatalf("RenderWaveGeneratePrompt(%s): %v", lang, err)
		}
		if strings.Contains(result, "Definition of Done") || strings.Contains(result, "完成基準") {
			t.Errorf("lang=%s: DoD section should not appear when empty", lang)
		}
	}
}

func TestRenderClassifyPromptWithLabels(t *testing.T) {
	data := domain.ClassifyPromptData{
		TeamFilter:      "test",
		ProjectFilter:   "test",
		OutputPath:      "/tmp/out.json",
		StrictnessLevel: "fog",
		LabelsEnabled:   true,
		LabelPrefix:     "sightjack",
	}
	for _, lang := range []string{"en", "ja"} {
		result, err := platform.RenderClassifyPrompt(lang, data)
		if err != nil {
			t.Fatalf("lang=%s: %v", lang, err)
		}
		if !strings.Contains(result, "sightjack:analyzed") {
			t.Errorf("lang=%s: expected label instruction in output", lang)
		}
	}
}

func TestRenderClassifyPromptWithoutLabels(t *testing.T) {
	data := domain.ClassifyPromptData{
		TeamFilter:      "test",
		ProjectFilter:   "test",
		OutputPath:      "/tmp/out.json",
		StrictnessLevel: "fog",
		LabelsEnabled:   false,
	}
	for _, lang := range []string{"en", "ja"} {
		result, err := platform.RenderClassifyPrompt(lang, data)
		if err != nil {
			t.Fatalf("lang=%s: %v", lang, err)
		}
		if strings.Contains(result, "analyzed") {
			t.Errorf("lang=%s: label instruction should not appear when disabled", lang)
		}
	}
}

func TestRenderWaveApplyPromptWithLabels(t *testing.T) {
	data := domain.WaveApplyPromptData{
		WaveID:          "w1",
		ClusterName:     "auth",
		Title:           "Wave 1",
		Actions:         "[]",
		OutputPath:      "/tmp/out.json",
		StrictnessLevel: "fog",
		LabelsEnabled:   true,
		LabelPrefix:     "sightjack",
	}
	for _, lang := range []string{"en", "ja"} {
		result, err := platform.RenderWaveApplyPrompt(lang, data)
		if err != nil {
			t.Fatalf("lang=%s: %v", lang, err)
		}
		if !strings.Contains(result, "sightjack:wave-done") {
			t.Errorf("lang=%s: expected label instruction in output", lang)
		}
	}
}

func TestRenderWaveApplyPromptNoReadySection(t *testing.T) {
	// given: wave_apply should NEVER contain ready-label instructions
	// (ready labels are applied in a separate step after apply success)
	data := domain.WaveApplyPromptData{
		WaveID:          "w1",
		ClusterName:     "auth",
		Title:           "Wave 1",
		Actions:         "[]",
		OutputPath:      "/tmp/out.json",
		StrictnessLevel: "fog",
		LabelsEnabled:   true,
		LabelPrefix:     "sightjack",
	}

	// when / then
	for _, lang := range []string{"en", "ja"} {
		result, err := platform.RenderWaveApplyPrompt(lang, data)
		if err != nil {
			t.Fatalf("lang=%s: %v", lang, err)
		}
		if strings.Contains(result, "ready") {
			t.Errorf("lang=%s: wave_apply must not contain ready-label section", lang)
		}
	}
}

func TestRenderWaveApplyPrompt_WithDoDSection(t *testing.T) {
	// given: wave apply prompt with DoD section
	data := domain.WaveApplyPromptData{
		WaveID:          "auth-w1",
		ClusterName:     "Auth",
		Title:           "Dependency Ordering",
		Actions:         `[{"type":"add_dod","issue_id":"ENG-101"}]`,
		OutputPath:      "/tmp/apply.json",
		StrictnessLevel: "alert",
		DoDSection:      "Must:\n- Unit tests required\nShould:\n- Integration tests\n",
	}

	// when / then
	for _, lang := range []string{"en", "ja"} {
		result, err := platform.RenderWaveApplyPrompt(lang, data)
		if err != nil {
			t.Fatalf("lang=%s: %v", lang, err)
		}
		if !strings.Contains(result, "Unit tests required") {
			t.Errorf("lang=%s: expected DoD Must item in prompt", lang)
		}
		if !strings.Contains(result, "Integration tests") {
			t.Errorf("lang=%s: expected DoD Should item in prompt", lang)
		}
	}
}

func TestRenderWaveApplyPrompt_WithoutDoDSection(t *testing.T) {
	// given: wave apply prompt without DoD section
	data := domain.WaveApplyPromptData{
		WaveID:          "auth-w1",
		ClusterName:     "Auth",
		Title:           "Dependency Ordering",
		Actions:         `[{"type":"add_dod","issue_id":"ENG-101"}]`,
		OutputPath:      "/tmp/apply.json",
		StrictnessLevel: "fog",
		DoDSection:      "",
	}

	// when / then
	for _, lang := range []string{"en", "ja"} {
		result, err := platform.RenderWaveApplyPrompt(lang, data)
		if err != nil {
			t.Fatalf("lang=%s: %v", lang, err)
		}
		if strings.Contains(result, "Definition of Done") || strings.Contains(result, "完成基準") {
			t.Errorf("lang=%s: DoD section should not appear when empty", lang)
		}
	}
}

func TestRenderWaveApplyPrompt_CreateActionDocumented(t *testing.T) {
	// given: wave apply prompt — template must document the create action type
	data := domain.WaveApplyPromptData{
		WaveID:          "auth-w2",
		ClusterName:     "Auth",
		Title:           "Sub-issue creation",
		Actions:         `[{"type":"create","issue_id":"ENG-100","description":"Create sub-issue for auth refactor","detail":"title: Auth token validation; parent: ENG-100"}]`,
		OutputPath:      "/tmp/apply_auth-w2.json",
		StrictnessLevel: "alert",
	}

	for _, lang := range []string{"en", "ja"} {
		t.Run(lang, func(t *testing.T) {
			// when
			result, err := platform.RenderWaveApplyPrompt(lang, data)

			// then
			if err != nil {
				t.Fatalf("render error: %v", err)
			}
			// Verify template Application Steps section documents the create action
			if !strings.Contains(result, "`create`") {
				t.Errorf("lang=%s: expected '`create`' action type in apply template steps", lang)
			}
			if lang == "en" && !strings.Contains(result, "sub-issue") {
				t.Errorf("lang=%s: expected 'sub-issue' in create action description", lang)
			}
			if lang == "ja" && !strings.Contains(result, "サブIssue") {
				t.Errorf("lang=%s: expected 'サブIssue' in create action description", lang)
			}
		})
	}
}

func TestRenderWaveGeneratePrompt_CreateActionDocumented(t *testing.T) {
	// given: wave generate prompt — template must document the create action type
	data := domain.WaveGeneratePromptData{
		ClusterName:     "Auth",
		Completeness:    "45",
		Issues:          `[{"id":"ENG-100","completeness":0.3}]`,
		Observations:    "Missing sub-tasks",
		OutputPath:      "/tmp/wave_auth.json",
		StrictnessLevel: "alert",
	}

	for _, lang := range []string{"en", "ja"} {
		t.Run(lang, func(t *testing.T) {
			// when
			result, err := platform.RenderWaveGeneratePrompt(lang, data)

			// then
			if err != nil {
				t.Fatalf("render error: %v", err)
			}
			if !strings.Contains(result, "`create`") {
				t.Errorf("lang=%s: expected 'create' action type in generate template", lang)
			}
		})
	}
}

func TestRenderReadyLabelPrompt(t *testing.T) {
	// given
	data := domain.ReadyLabelPromptData{
		ReadyLabel:    "sightjack:ready",
		ReadyIssueIDs: "AUTH-1, AUTH-2",
	}

	// when / then
	for _, lang := range []string{"en", "ja"} {
		result, err := platform.RenderReadyLabelPrompt(lang, data)
		if err != nil {
			t.Fatalf("lang=%s: %v", lang, err)
		}
		if !strings.Contains(result, "sightjack:ready") {
			t.Errorf("lang=%s: expected ready label in output", lang)
		}
		if !strings.Contains(result, "AUTH-1") {
			t.Errorf("lang=%s: expected issue IDs in output", lang)
		}
	}
}

func TestRenderReadyLabelPrompt_UnsupportedLang(t *testing.T) {
	// given
	data := domain.ReadyLabelPromptData{
		ReadyLabel:    "sightjack:ready",
		ReadyIssueIDs: "AUTH-1",
	}

	// when
	_, err := platform.RenderReadyLabelPrompt("fr", data)

	// then
	if err == nil {
		t.Fatal("expected error for unsupported language")
	}
}

func TestRenderNextGenPromptWithDoD(t *testing.T) {
	// given
	data := domain.NextGenPromptData{
		ClusterName:     "auth",
		Completeness:    "70",
		Issues:          "[]",
		CompletedWaves:  "[]",
		OutputPath:      "/tmp/out.json",
		StrictnessLevel: "fog",
		DoDSection:      "Must:\n- Terraform reviewed\n",
	}

	// when / then
	for _, lang := range []string{"en", "ja"} {
		result, err := platform.RenderNextGenPrompt(lang, data)
		if err != nil {
			t.Fatalf("RenderNextGenPrompt(%s): %v", lang, err)
		}
		if !strings.Contains(result, "Terraform reviewed") {
			t.Errorf("lang=%s: expected DoD section in output", lang)
		}
	}
}

func TestRenderNextGenPrompt_WithFeedback(t *testing.T) {
	// given
	data := domain.NextGenPromptData{
		ClusterName:     "Auth",
		Completeness:    "60",
		Issues:          `[]`,
		CompletedWaves:  `[]`,
		FeedbackSection: "### [HIGH] fb-001\nArchitecture drift detected\n",
		OutputPath:      "/tmp/out.json",
		StrictnessLevel: "fog",
	}

	// when / then: both languages should render feedback
	for _, lang := range []string{"en", "ja"} {
		result, err := platform.RenderNextGenPrompt(lang, data)
		if err != nil {
			t.Fatalf("RenderNextGenPrompt(%s): %v", lang, err)
		}
		if !strings.Contains(result, "fb-001") {
			t.Errorf("lang=%s: expected feedback name in output", lang)
		}
		if !strings.Contains(result, "Architecture drift detected") {
			t.Errorf("lang=%s: expected feedback content in output", lang)
		}
		if !strings.Contains(result, "[HIGH]") {
			t.Errorf("lang=%s: expected HIGH severity marker", lang)
		}
	}
}

func TestRenderNextGenPrompt_NoFeedback(t *testing.T) {
	// given: no feedback
	data := domain.NextGenPromptData{
		ClusterName:     "DB",
		Completeness:    "40",
		Issues:          `[]`,
		CompletedWaves:  `[]`,
		OutputPath:      "/tmp/out.json",
		StrictnessLevel: "fog",
	}

	// when
	for _, lang := range []string{"en", "ja"} {
		result, err := platform.RenderNextGenPrompt(lang, data)
		if err != nil {
			t.Fatalf("RenderNextGenPrompt(%s): %v", lang, err)
		}
		if strings.Contains(result, "Received Feedback") || strings.Contains(result, "受信フィードバック") {
			t.Errorf("lang=%s: feedback section should be omitted when empty", lang)
		}
	}
}
