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

func TestRenderNavigatorWithWaves(t *testing.T) {
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
	nav := RenderNavigatorWithWaves(result, "TestProject", waves, 0, nil)

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

func TestRenderNavigatorWithWaves_CompletedWave(t *testing.T) {
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
	nav := RenderNavigatorWithWaves(result, "TestProject", waves, 0, nil)

	// then
	if !strings.Contains(nav, "[=]") {
		t.Error("expected [=] for completed wave")
	}
}

func TestRenderNavigatorWithWaves_ADRCountZero(t *testing.T) {
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
	nav := RenderNavigatorWithWaves(result, "TestProject", waves, 0, nil)

	// then
	if !strings.Contains(nav, "ADRs: 0") {
		t.Error("expected 'ADRs: 0' in navigator")
	}
}

func TestRenderNavigatorWithWaves_ADRCountPositive(t *testing.T) {
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
	nav := RenderNavigatorWithWaves(result, "TestProject", waves, 5, nil)

	// then
	if !strings.Contains(nav, "ADRs: 5") {
		t.Error("expected 'ADRs: 5' in navigator")
	}
}

func TestRenderNavigatorWithWaves_ResumeInfo(t *testing.T) {
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
	nav := RenderNavigatorWithWaves(result, "TestProject", waves, 3, &lastScanned)

	// then
	if !strings.Contains(nav, "Session: resumed") {
		t.Error("expected 'Session: resumed' in navigator")
	}
	if !strings.Contains(nav, "2026-02-17 15:30") {
		t.Error("expected last scan timestamp in navigator")
	}
}

func TestRenderNavigatorWithWaves_NoResumeInfo(t *testing.T) {
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
	nav := RenderNavigatorWithWaves(result, "TestProject", waves, 0, nil)

	// then: no resume line
	if strings.Contains(nav, "Session:") {
		t.Error("should not contain 'Session:' for fresh session")
	}
}
