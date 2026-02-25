package session_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hironow/sightjack"
	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/session"
)

func TestParseWaveGenerateResult(t *testing.T) {
	// given
	dir := t.TempDir()
	path := filepath.Join(dir, "wave_auth.json")
	content := `{
		"cluster_name": "Auth",
		"waves": [
			{"id": "auth-w1", "cluster_name": "Auth", "title": "Deps", "actions": [], "prerequisites": [], "delta": {"before": 0.25, "after": 0.40}, "status": "available"}
		]
	}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// when
	result, err := session.ParseWaveGenerateResult(path)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ClusterName != "Auth" {
		t.Errorf("expected Auth, got %s", result.ClusterName)
	}
	if len(result.Waves) != 1 {
		t.Fatalf("expected 1 wave, got %d", len(result.Waves))
	}
}

func TestParseWaveApplyResult(t *testing.T) {
	// given
	dir := t.TempDir()
	path := filepath.Join(dir, "apply_auth-w1.json")
	content := `{
		"wave_id": "auth-w1",
		"applied": 5,
		"errors": [],
		"ripples": [{"cluster_name": "API", "description": "W2 unlocked"}]
	}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// when
	result, err := session.ParseWaveApplyResult(path)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Applied != 5 {
		t.Errorf("expected 5, got %d", result.Applied)
	}
}

func TestAvailableWaves(t *testing.T) {
	// given
	waves := []sightjack.Wave{
		{ID: "auth-w1", ClusterName: "Auth", Status: "available", Prerequisites: nil},
		{ID: "auth-w2", ClusterName: "Auth", Status: "locked", Prerequisites: []string{"Auth:auth-w1"}},
		{ID: "api-w1", ClusterName: "API", Status: "available", Prerequisites: nil},
		{ID: "api-w2", ClusterName: "API", Status: "locked", Prerequisites: []string{"API:api-w1", "Auth:auth-w1"}},
	}
	completed := map[string]bool{}

	// when
	available := domain.AvailableWaves(waves, completed)

	// then
	if len(available) != 2 {
		t.Fatalf("expected 2 available, got %d", len(available))
	}

	// given: after completing auth-w1
	completed[domain.WaveKey(waves[0])] = true
	waves = domain.EvaluateUnlocks(waves, completed)

	// when
	available = domain.AvailableWaves(waves, completed)

	// then: auth-w2 should be unlocked now (prereq Auth:auth-w1 met)
	// api-w2 still locked (needs API:api-w1 too)
	if len(available) != 2 {
		t.Fatalf("expected 2 available after auth-w1 complete, got %d", len(available))
	}
	found := false
	for _, w := range available {
		if w.ID == "auth-w2" {
			found = true
		}
	}
	if !found {
		t.Error("expected auth-w2 to be available")
	}
}

func TestMergeWaveResults(t *testing.T) {
	results := []sightjack.WaveGenerateResult{
		{ClusterName: "Auth", Waves: []sightjack.Wave{{ID: "auth-w1"}, {ID: "auth-w2"}}},
		{ClusterName: "API", Waves: []sightjack.Wave{{ID: "api-w1"}}},
	}

	merged := domain.MergeWaveResults(results)
	if len(merged) != 3 {
		t.Fatalf("expected 3 waves, got %d", len(merged))
	}
}

func TestMergeWaveResults_Empty(t *testing.T) {
	merged := domain.MergeWaveResults(nil)
	if len(merged) != 0 {
		t.Errorf("expected 0 waves, got %d", len(merged))
	}
}

func TestWaveApplyFileName(t *testing.T) {
	// given
	wave := sightjack.Wave{ID: "auth-w1", ClusterName: "Auth"}

	// when
	got := session.WaveApplyFileName(wave)

	// then
	expected := "apply_auth_auth-w1.json"
	if got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestWaveApplyFileName_SpecialChars(t *testing.T) {
	// given
	wave := sightjack.Wave{ID: "w2", ClusterName: "My Cluster"}

	// when
	got := session.WaveApplyFileName(wave)

	// then
	expected := "apply_my_cluster_w2.json"
	if got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestWaveApplyFileName_DuplicateIDsDifferentClusters(t *testing.T) {
	// given: two waves with same ID but different clusters
	authWave := sightjack.Wave{ID: "w1", ClusterName: "Auth"}
	apiWave := sightjack.Wave{ID: "w1", ClusterName: "API"}

	// when
	authFile := session.WaveApplyFileName(authWave)
	apiFile := session.WaveApplyFileName(apiWave)

	// then: filenames must differ
	if authFile == apiFile {
		t.Errorf("duplicate filenames for different clusters: %s", authFile)
	}
}

func TestWaveKey(t *testing.T) {
	// given
	w := sightjack.Wave{ID: "w1", ClusterName: "Auth"}

	// when
	key := domain.WaveKey(w)

	// then
	if key != "Auth:w1" {
		t.Errorf("expected Auth:w1, got %s", key)
	}
}

func TestAvailableWaves_DuplicateIDsAcrossClusters(t *testing.T) {
	// given: two clusters with the same wave ID "w1"
	waves := []sightjack.Wave{
		{ID: "w1", ClusterName: "Auth", Status: "available"},
		{ID: "w1", ClusterName: "API", Status: "available"},
	}
	// only Auth:w1 is completed
	completed := map[string]bool{domain.WaveKey(waves[0]): true}

	// when
	available := domain.AvailableWaves(waves, completed)

	// then: API:w1 should still be available
	if len(available) != 1 {
		t.Fatalf("expected 1 available, got %d", len(available))
	}
	if available[0].ClusterName != "API" {
		t.Errorf("expected API cluster wave, got %s", available[0].ClusterName)
	}
}

func TestEvaluateUnlocks_DuplicateIDsAcrossClusters(t *testing.T) {
	// given: Auth:w1 completed, API:w1 locked with prereq Auth:w1
	waves := []sightjack.Wave{
		{ID: "w1", ClusterName: "Auth", Status: "completed"},
		{ID: "w1", ClusterName: "API", Status: "locked", Prerequisites: []string{"Auth:w1"}},
	}
	completed := map[string]bool{domain.WaveKey(waves[0]): true}

	// when
	updated := domain.EvaluateUnlocks(waves, completed)

	// then: API:w1 should be unlocked
	if updated[1].Status != "available" {
		t.Errorf("expected API:w1 available, got %s", updated[1].Status)
	}
}

func TestNormalizeWavePrerequisites(t *testing.T) {
	// given: bare IDs should be prefixed with the wave's own cluster name
	waves := []sightjack.Wave{
		{ID: "w1", ClusterName: "Auth", Prerequisites: nil},
		{ID: "w2", ClusterName: "Auth", Prerequisites: []string{"w1"}},
		{ID: "w1", ClusterName: "API", Prerequisites: []string{"Auth:w1"}},
	}

	// when
	normalized := domain.NormalizeWavePrerequisites(waves)

	// then: bare "w1" becomes "Auth:w1", explicit "Auth:w1" stays
	if len(normalized[0].Prerequisites) != 0 {
		t.Errorf("w1 should have no prereqs")
	}
	if normalized[1].Prerequisites[0] != "Auth:w1" {
		t.Errorf("expected Auth:w1, got %s", normalized[1].Prerequisites[0])
	}
	if normalized[2].Prerequisites[0] != "Auth:w1" {
		t.Errorf("expected Auth:w1, got %s", normalized[2].Prerequisites[0])
	}
}

func TestAvailableWaves_AllCompleted(t *testing.T) {
	// given: all waves are completed
	waves := []sightjack.Wave{
		{ID: "auth-w1", ClusterName: "Auth", Status: "completed"},
		{ID: "auth-w2", ClusterName: "Auth", Status: "completed"},
		{ID: "api-w1", ClusterName: "API", Status: "completed"},
	}
	completed := map[string]bool{
		"Auth:auth-w1": true,
		"Auth:auth-w2": true,
		"API:api-w1":   true,
	}

	// when
	available := domain.AvailableWaves(waves, completed)

	// then: no waves should be available — session is done
	if len(available) != 0 {
		t.Errorf("expected 0 available waves when all completed, got %d", len(available))
	}
}

func TestEvaluateUnlocks_AllCompleted(t *testing.T) {
	// given: all waves already completed, nothing left to unlock
	waves := []sightjack.Wave{
		{ID: "a-w1", ClusterName: "A", Status: "completed"},
		{ID: "a-w2", ClusterName: "A", Status: "completed", Prerequisites: []string{"A:a-w1"}},
		{ID: "b-w1", ClusterName: "B", Status: "completed", Prerequisites: []string{"A:a-w1"}},
	}
	completed := map[string]bool{
		"A:a-w1": true,
		"A:a-w2": true,
		"B:b-w1": true,
	}

	// when
	updated := domain.EvaluateUnlocks(waves, completed)

	// then: all remain completed, no status changes
	for _, w := range updated {
		if w.Status != "completed" {
			t.Errorf("expected %s to remain completed, got %s", domain.WaveKey(w), w.Status)
		}
	}
}

func TestToApplyResult_ZeroActions_ReturnsBeforeCompleteness(t *testing.T) {
	// given: wave with no actions — nothing to accomplish
	wave := sightjack.Wave{
		ID:          "empty-w1",
		ClusterName: "Auth",
		Delta:       sightjack.WaveDelta{Before: 0.3, After: 0.5},
		Actions:     nil,
	}
	internal := &sightjack.WaveApplyResult{WaveID: "empty-w1", Applied: 0}

	// when
	result := session.ToApplyResult(wave, internal)

	// then: no actions means nothing accomplished → completeness should be Before
	if result.NewCompleteness != 0.3 {
		t.Errorf("expected 0.3 (Delta.Before), got %f", result.NewCompleteness)
	}
}

func TestEvaluateUnlocks(t *testing.T) {
	// given
	waves := []sightjack.Wave{
		{ID: "a-w1", ClusterName: "A", Status: "completed"},
		{ID: "a-w2", ClusterName: "A", Status: "locked", Prerequisites: []string{"A:a-w1"}},
		{ID: "b-w1", ClusterName: "B", Status: "locked", Prerequisites: []string{"A:a-w1", "A:a-w2"}},
	}
	completed := map[string]bool{"A:a-w1": true}

	// when
	updated := domain.EvaluateUnlocks(waves, completed)

	// then
	if updated[1].Status != "available" {
		t.Errorf("expected a-w2 available, got %s", updated[1].Status)
	}
	if updated[2].Status != "locked" {
		t.Errorf("expected b-w1 still locked, got %s", updated[2].Status)
	}
}
