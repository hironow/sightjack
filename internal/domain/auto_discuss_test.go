package domain

import "testing"

func TestAutoDiscussResult_ToArchitectResponse(t *testing.T) {
	// given
	result := AutoDiscussResult{
		Rounds: []AutoDiscussRound{
			{Round: 0, Speaker: "architect", Content: "The wave restructures auth module."},
			{Round: 1, Speaker: "devils_advocate", Content: "This contradicts ADR 0003."},
			{Round: 1, Speaker: "architect", Content: "ADR 0003 should be superseded."},
		},
		OpenIssues: []string{"ADR 0003 supersession not documented"},
		Summary:    "Auth module restructure requires ADR update.",
	}

	// when
	resp := result.ToArchitectResponse()

	// then
	if resp.Analysis == "" {
		t.Error("expected non-empty Analysis")
	}
	if resp.Reasoning == "" {
		t.Error("expected non-empty Reasoning")
	}
	if resp.Decision == "" {
		t.Error("expected non-empty Decision")
	}
	if resp.ModifiedWave != nil {
		t.Error("expected nil ModifiedWave")
	}
}

func TestAutoDiscussResult_ToArchitectResponse_Empty(t *testing.T) {
	// given
	result := AutoDiscussResult{}

	// when
	resp := result.ToArchitectResponse()

	// then
	if resp.Decision == "" {
		t.Error("expected fallback Decision for empty result")
	}
	if resp.ModifiedWave != nil {
		t.Error("expected nil ModifiedWave")
	}
}

func TestDefaultConfig_AutoDiscussRounds(t *testing.T) {
	// given/when
	cfg := DefaultConfig()

	// then
	if cfg.Scribe.AutoDiscussRounds != 2 {
		t.Errorf("expected default AutoDiscussRounds=2, got %d", cfg.Scribe.AutoDiscussRounds)
	}
}
