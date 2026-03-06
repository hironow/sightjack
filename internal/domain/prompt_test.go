package domain_test

import (
	"strings"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
)

func TestMatchDoDTemplate(t *testing.T) {
	templates := map[string]domain.DoDTemplate{
		"auth":  {Must: []string{"auth must"}, Should: []string{"auth should"}},
		"infra": {Must: []string{"infra must"}},
	}
	tests := []struct {
		clusterName string
		wantMatch   bool
		wantKey     string
	}{
		{"Auth", true, "auth"},
		{"auth-service", true, "auth"},
		{"Authentication", true, "auth"},
		{"INFRA", true, "infra"},
		{"frontend", false, ""},
	}
	for _, tt := range tests {
		matched, key := domain.MatchDoDTemplate(templates, tt.clusterName)
		if matched != tt.wantMatch {
			t.Errorf("MatchDoDTemplate(%q): matched=%v, want %v", tt.clusterName, matched, tt.wantMatch)
		}
		if key != tt.wantKey {
			t.Errorf("MatchDoDTemplate(%q): key=%q, want %q", tt.clusterName, key, tt.wantKey)
		}
	}
}

func TestMatchDoDTemplate_CaseTieBreaker(t *testing.T) {
	// given: keys that differ only by case -> same length after lowering
	templates := map[string]domain.DoDTemplate{
		"Auth": {Must: []string{"upper"}},
		"auth": {Must: []string{"lower"}},
	}

	// when: both match "authentication" with equal length prefix
	matched, key := domain.MatchDoDTemplate(templates, "authentication")

	// then: "Auth" < "auth" in ASCII, so "Auth" wins the original-key tie-breaker
	if !matched {
		t.Fatal("expected a match")
	}
	if key != "Auth" {
		t.Errorf("expected deterministic winner 'Auth', got %q", key)
	}

	// Verify determinism across multiple calls (map iteration order varies)
	for i := range 50 {
		_, k := domain.MatchDoDTemplate(templates, "authentication")
		if k != key {
			t.Fatalf("non-deterministic on iteration %d: got %q, want %q", i, k, key)
		}
	}
}

func TestMatchDoDTemplate_LongestPrefixWins(t *testing.T) {
	// given: "a" and "auth" both match "authentication"
	templates := map[string]domain.DoDTemplate{
		"a":    {Must: []string{"short"}},
		"auth": {Must: []string{"long"}},
	}

	// when
	matched, key := domain.MatchDoDTemplate(templates, "authentication")

	// then: longest prefix wins
	if !matched {
		t.Fatal("expected a match")
	}
	if key != "auth" {
		t.Errorf("expected longest prefix 'auth', got %q", key)
	}
}

func TestFormatDoDSection(t *testing.T) {
	tmpl := domain.DoDTemplate{
		Must:   []string{"Unit tests", "Error handling"},
		Should: []string{"Integration tests"},
	}
	section := domain.FormatDoDSection(tmpl)
	if !strings.Contains(section, "Unit tests") {
		t.Error("expected Must items in section")
	}
	if !strings.Contains(section, "Integration tests") {
		t.Error("expected Should items in section")
	}
}

func TestFormatDoDSectionEmpty(t *testing.T) {
	section := domain.FormatDoDSection(domain.DoDTemplate{})
	if section != "" {
		t.Errorf("expected empty section for empty template, got %q", section)
	}
}
