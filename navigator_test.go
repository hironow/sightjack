package sightjack

import (
	"strings"
	"testing"
	"time"
)

func TestRenderNavigator_Basic(t *testing.T) {
	result := &ScanResult{
		Clusters: []ClusterScanResult{
			{Name: "Auth", Completeness: 0.25, Issues: make([]IssueDetail, 5)},
			{Name: "API", Completeness: 0.40, Issues: make([]IssueDetail, 8)},
		},
		TotalIssues:  13,
		Completeness: 0.325,
	}

	output := RenderNavigator(result, "My Project")

	if !strings.Contains(output, "SIGHTJACK") {
		t.Error("expected SIGHTJACK header")
	}
	if !strings.Contains(output, "My Project") {
		t.Error("expected project name")
	}
	if !strings.Contains(output, "Auth") {
		t.Error("expected Auth cluster")
	}
	if !strings.Contains(output, "API") {
		t.Error("expected API cluster")
	}
	if !strings.Contains(output, "25%") {
		t.Error("expected Auth completeness 25%")
	}
	if !strings.Contains(output, "40%") {
		t.Error("expected API completeness 40%")
	}
	if !strings.Contains(output, "32%") {
		t.Error("expected overall completeness ~32%")
	}
}

func TestRenderNavigator_Empty(t *testing.T) {
	result := &ScanResult{}

	output := RenderNavigator(result, "Empty Project")

	if !strings.Contains(output, "SIGHTJACK") {
		t.Error("expected SIGHTJACK header even with no clusters")
	}
	if !strings.Contains(output, "0%") {
		t.Error("expected 0% completeness")
	}
}

func TestDisplayWidth(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"hello", 5},
		{"認証", 4},
		{"Auth認証", 8},
		{"", 0},
		{"W1  W2", 6},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := displayWidth(tt.input)
			if got != tt.expected {
				t.Errorf("displayWidth(%q) = %d, want %d", tt.input, got, tt.expected)
			}
		})
	}
}

func TestTruncate_Japanese(t *testing.T) {
	// given: Japanese text longer than maxWidth in display columns
	// Each CJK char is 2 display columns, so 5 cols fits 2 chars (4 cols) + "~" (1 col) = 5
	input := "認証とアクセス制御"

	// when
	result := truncate(input, 5)

	// then: should truncate at display width boundary
	if result != "認証~" {
		t.Errorf("truncate(%q, 5) = %q, want %q", input, result, "認証~")
	}
}

func TestTruncate_ASCII(t *testing.T) {
	// given
	input := "Authentication"

	// when
	result := truncate(input, 8)

	// then
	if result != "Authent~" {
		t.Errorf("truncate(%q, 8) = %q, want %q", input, result, "Authent~")
	}
}

func TestTruncate_Short(t *testing.T) {
	// given: string within limit
	input := "Auth"

	// when
	result := truncate(input, 10)

	// then: should return original
	if result != "Auth" {
		t.Errorf("truncate(%q, 10) = %q, want %q", input, result, "Auth")
	}
}

func TestCenter_Japanese(t *testing.T) {
	// given: Japanese text to center (displayWidth "認証" = 4)
	input := "認証"

	// when: center in 10 display columns
	result := center(input, 10)

	// then: should pad correctly based on display width
	// pad = (10-4)/2 = 3 on each side
	if displayWidth(result) != 10 {
		t.Errorf("center(%q, 10) display width = %d, want 10", input, displayWidth(result))
	}
	if result != "   認証   " {
		t.Errorf("center(%q, 10) = %q, want %q", input, result, "   認証   ")
	}
}

func TestRenderNavigator_ConsistentLineWidth(t *testing.T) {
	// given
	result := &ScanResult{
		Clusters: []ClusterScanResult{
			{Name: "Auth", Completeness: 0.25, Issues: make([]IssueDetail, 3)},
		},
		TotalIssues:  3,
		Completeness: 0.25,
	}

	// when
	output := RenderNavigator(result, "My Project")

	// then: every non-empty line must have the same display width
	lines := strings.Split(output, "\n")
	expectedWidth := 2 + navigatorWidth // "|" or "+" on each side
	for i, line := range lines {
		if line == "" {
			continue
		}
		dw := displayWidth(line)
		if dw != expectedWidth {
			t.Errorf("line %d: display width %d, want %d: %q", i+1, dw, expectedWidth, line)
		}
	}
}

func TestRenderNavigator_JapaneseName(t *testing.T) {
	// given: Japanese project and cluster names
	result := &ScanResult{
		Clusters: []ClusterScanResult{
			{Name: "認証", Completeness: 0.5, Issues: make([]IssueDetail, 3)},
		},
		TotalIssues:  3,
		Completeness: 0.5,
	}

	// when
	output := RenderNavigator(result, "テストプロジェクト")

	// then: every non-empty line must have consistent display width
	lines := strings.Split(output, "\n")
	expectedWidth := 2 + navigatorWidth
	for i, line := range lines {
		if line == "" {
			continue
		}
		dw := displayWidth(line)
		if dw != expectedWidth {
			t.Errorf("line %d: display width %d, want %d: %q", i+1, dw, expectedWidth, line)
		}
	}
}

func TestRenderNavigator_LongClusterName(t *testing.T) {
	result := &ScanResult{
		Clusters: []ClusterScanResult{
			{Name: "Authentication & Authorization", Completeness: 0.5},
		},
		Completeness: 0.5,
	}

	output := RenderNavigator(result, "Test")

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if len(line) > 80 {
			t.Logf("Long line (%d chars): %s", len(line), line)
		}
	}
}

func TestRenderMatrixNavigator_Basic(t *testing.T) {
	// given
	result := &ScanResult{
		Clusters: []ClusterScanResult{
			{Name: "Auth", Completeness: 0.25},
			{Name: "API", Completeness: 0.30},
		},
		TotalIssues:  10,
		Completeness: 0.275,
	}
	waves := []Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Deps", Status: "available"},
		{ID: "auth-w2", ClusterName: "Auth", Title: "DoD", Status: "locked"},
		{ID: "api-w1", ClusterName: "API", Title: "Split", Status: "available"},
	}

	// when
	nav := RenderMatrixNavigator(result, "TestProject", waves, 0, nil, "fog", 0)

	// then
	if !strings.Contains(nav, "[ ]") {
		t.Error("expected [ ] for available wave")
	}
	if !strings.Contains(nav, "[x]") {
		t.Error("expected [x] for locked wave")
	}
	if !strings.Contains(nav, "Deps") {
		t.Error("expected wave title 'Deps' in output")
	}
}

func TestRenderMatrixNavigator_CompletedWave(t *testing.T) {
	// given
	result := &ScanResult{
		Clusters:     []ClusterScanResult{{Name: "Auth", Completeness: 0.40}},
		TotalIssues:  4,
		Completeness: 0.40,
	}
	waves := []Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Deps", Status: "completed"},
		{ID: "auth-w2", ClusterName: "Auth", Title: "DoD", Status: "available"},
	}

	// when
	nav := RenderMatrixNavigator(result, "TestProject", waves, 0, nil, "fog", 0)

	// then
	if !strings.Contains(nav, "[=]") {
		t.Error("expected [=] for completed wave")
	}
}

func TestRenderMatrixNavigator_ADRCountZero(t *testing.T) {
	// given
	result := &ScanResult{
		Clusters:     []ClusterScanResult{{Name: "Auth", Completeness: 0.25}},
		TotalIssues:  3,
		Completeness: 0.25,
	}
	waves := []Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Deps", Status: "available"},
	}

	// when
	nav := RenderMatrixNavigator(result, "TestProject", waves, 0, nil, "fog", 0)

	// then
	if !strings.Contains(nav, "ADR: 0") {
		t.Error("expected 'ADR: 0' in navigator footer")
	}
}

func TestRenderMatrixNavigator_ADRCountPositive(t *testing.T) {
	// given
	result := &ScanResult{
		Clusters:     []ClusterScanResult{{Name: "Auth", Completeness: 0.50}},
		TotalIssues:  5,
		Completeness: 0.50,
	}
	waves := []Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Deps", Status: "completed"},
	}

	// when
	nav := RenderMatrixNavigator(result, "TestProject", waves, 5, nil, "fog", 0)

	// then
	if !strings.Contains(nav, "ADR: 5") {
		t.Error("expected 'ADR: 5' in navigator footer")
	}
}

func TestRenderMatrixNavigator_ResumeInfo(t *testing.T) {
	// given
	result := &ScanResult{
		Clusters:     []ClusterScanResult{{Name: "Auth", Completeness: 0.62}},
		TotalIssues:  5,
		Completeness: 0.62,
	}
	waves := []Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Deps", Status: "completed"},
	}
	lastScanned := time.Date(2026, 2, 17, 15, 30, 0, 0, time.UTC)

	// when
	nav := RenderMatrixNavigator(result, "TestProject", waves, 3, &lastScanned, "fog", 0)

	// then
	if !strings.Contains(nav, "Session: resumed") {
		t.Error("expected 'Session: resumed' in header")
	}
	if !strings.Contains(nav, "2026-02-17 15:30") {
		t.Error("expected last scan timestamp in header")
	}
}

func TestRenderMatrixNavigator_NoResumeInfo(t *testing.T) {
	// given: nil lastScanned means fresh session
	result := &ScanResult{
		Clusters:     []ClusterScanResult{{Name: "Auth", Completeness: 0.25}},
		TotalIssues:  3,
		Completeness: 0.25,
	}
	waves := []Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Deps", Status: "available"},
	}

	// when
	nav := RenderMatrixNavigator(result, "TestProject", waves, 0, nil, "fog", 0)

	// then: no resume line
	if strings.Contains(nav, "Session:") {
		t.Error("should not contain 'Session:' for fresh session")
	}
}

func TestRenderMatrixNavigator_StrictnessBadge(t *testing.T) {
	// given
	result := &ScanResult{
		Clusters:     []ClusterScanResult{{Name: "Auth", Completeness: 0.25}},
		TotalIssues:  3,
		Completeness: 0.25,
	}
	waves := []Wave{{ID: "auth-w1", ClusterName: "Auth", Title: "Deps", Status: "available"}}

	// when
	nav := RenderMatrixNavigator(result, "TestProject", waves, 0, nil, "alert", 0)

	// then
	if !strings.Contains(nav, "Strictness: alert") {
		t.Error("expected 'Strictness: alert' in footer")
	}
}

func TestRenderMatrixNavigator_ShibitoCount(t *testing.T) {
	// given
	result := &ScanResult{
		Clusters:     []ClusterScanResult{{Name: "Auth", Completeness: 0.25}},
		TotalIssues:  3,
		Completeness: 0.25,
	}
	waves := []Wave{{ID: "auth-w1", ClusterName: "Auth", Title: "Deps", Status: "available"}}

	// when
	nav := RenderMatrixNavigator(result, "TestProject", waves, 0, nil, "fog", 3)

	// then
	if !strings.Contains(nav, "Shibito: 3") {
		t.Error("expected 'Shibito: 3' in header")
	}
}

func TestRenderProgressBar_Half(t *testing.T) {
	// given
	current := 0.50

	// when
	result := RenderProgressBar(current, 20)

	// then
	expected := "[==========..........] 50%"
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}

func TestRenderProgressBar_Zero(t *testing.T) {
	// given / when
	result := RenderProgressBar(0.0, 20)

	// then
	expected := "[....................] 0%"
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}

func TestRenderProgressBar_Full(t *testing.T) {
	// given / when
	result := RenderProgressBar(1.0, 20)

	// then
	expected := "[====================] 100%"
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}

func TestRenderProgressBar_Partial(t *testing.T) {
	// given: 62% with width 20 -> 12.4 -> 12 filled
	result := RenderProgressBar(0.62, 20)

	// then
	expected := "[============........] 62%"
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}

func TestRenderProgressBar_Overflow(t *testing.T) {
	// given: current > 1.0 should clamp to 100%
	result := RenderProgressBar(1.5, 20)

	// then
	expected := "[====================] 100%"
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}

func TestRenderProgressBar_Underflow(t *testing.T) {
	// given: negative current should clamp to 0%
	result := RenderProgressBar(-0.25, 20)

	// then
	expected := "[....................] 0%"
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}

func TestRenderMatrixNavigator_ShibitoZero_Hidden(t *testing.T) {
	// given
	result := &ScanResult{
		Clusters:     []ClusterScanResult{{Name: "Auth", Completeness: 0.25}},
		TotalIssues:  3,
		Completeness: 0.25,
	}
	waves := []Wave{{ID: "auth-w1", ClusterName: "Auth", Title: "Deps", Status: "available"}}

	// when
	nav := RenderMatrixNavigator(result, "TestProject", waves, 0, nil, "fog", 0)

	// then: shibito count should not appear when 0
	if strings.Contains(nav, "Shibito") {
		t.Error("should not show 'Shibito' when count is 0")
	}
}

func TestRenderMatrixNavigator_GridBorders(t *testing.T) {
	result := &ScanResult{
		Clusters: []ClusterScanResult{
			{Name: "Auth", Completeness: 0.65, Issues: make([]IssueDetail, 4)},
			{Name: "API", Completeness: 0.58, Issues: make([]IssueDetail, 6)},
		},
		TotalIssues:  10,
		Completeness: 0.615,
	}
	waves := []Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Deps", Status: "completed"},
		{ID: "auth-w2", ClusterName: "Auth", Title: "DoD", Status: "available"},
		{ID: "api-w1", ClusterName: "API", Title: "Split", Status: "completed"},
	}
	nav := RenderMatrixNavigator(result, "TestProject", waves, 4, nil, "fog", 0)
	if !strings.Contains(nav, "+--") {
		t.Error("expected '+--' grid border")
	}
	if !strings.Contains(nav, "| Cluster") {
		t.Error("expected '| Cluster' header row")
	}
	if !strings.Contains(nav, "| W1") {
		t.Error("expected '| W1' column header")
	}
	if !strings.Contains(nav, "[=]") {
		t.Error("expected [=] for completed wave")
	}
	if !strings.Contains(nav, "[ ]") {
		t.Error("expected [ ] for available wave")
	}
	if !strings.Contains(nav, "61%") {
		t.Error("expected 61% in progress bar")
	}
}

func TestRenderMatrixNavigator_JapaneseClusterAlignment(t *testing.T) {
	// given: Japanese cluster name (wide characters) should not break grid alignment
	result := &ScanResult{
		Clusters: []ClusterScanResult{
			{Name: "認証", Completeness: 0.50, Issues: make([]IssueDetail, 3)},
			{Name: "API", Completeness: 0.40, Issues: make([]IssueDetail, 2)},
		},
		TotalIssues:  5,
		Completeness: 0.45,
	}
	waves := []Wave{
		{ID: "w1", ClusterName: "認証", Title: "Deps", Status: "available"},
		{ID: "w2", ClusterName: "API", Title: "Split", Status: "completed"},
	}

	// when
	nav := RenderMatrixNavigator(result, "テストプロジェクト", waves, 0, nil, "fog", 0)

	// then: all grid lines (starting with + or |) must have the same display width
	lines := strings.Split(nav, "\n")
	var gridWidth int
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		if line[0] != '+' && line[0] != '|' {
			continue
		}
		dw := displayWidth(line)
		if gridWidth == 0 {
			gridWidth = dw
		}
		if dw != gridWidth {
			t.Errorf("grid line display width %d, want %d: %q", dw, gridWidth, line)
		}
	}
}

func TestRenderMatrixNavigator_ProgressBarInFooter(t *testing.T) {
	result := &ScanResult{
		Clusters:     []ClusterScanResult{{Name: "Auth", Completeness: 0.50}},
		Completeness: 0.50,
	}
	waves := []Wave{{ID: "w1", ClusterName: "Auth", Title: "T", Status: "available"}}
	nav := RenderMatrixNavigator(result, "P", waves, 2, nil, "alert", 0)
	if !strings.Contains(nav, "ADR: 2") {
		t.Error("expected ADR count in footer")
	}
	if !strings.Contains(nav, "Strictness: alert") {
		t.Error("expected strictness in footer")
	}
	if !strings.Contains(nav, "50%") {
		t.Error("expected 50% in progress bar")
	}
}
