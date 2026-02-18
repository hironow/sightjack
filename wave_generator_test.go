package sightjack

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNextgenFileName(t *testing.T) {
	wave := Wave{ClusterName: "Auth", ID: "auth-w2"}
	got := nextgenFileName(wave)
	want := "nextgen_auth_auth-w2.json"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestClearNextgenOutput_RemovesFile(t *testing.T) {
	dir := t.TempDir()
	wave := Wave{ClusterName: "Auth", ID: "auth-w1"}
	path := filepath.Join(dir, nextgenFileName(wave))
	if err := os.WriteFile(path, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	clearNextgenOutput(dir, wave)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("file should have been removed")
	}
}

func TestClearNextgenOutput_NoopIfMissing(t *testing.T) {
	dir := t.TempDir()
	wave := Wave{ClusterName: "Auth", ID: "auth-w1"}
	clearNextgenOutput(dir, wave) // should not panic
}

func TestParseNextGenResult_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nextgen.json")
	data := `{"cluster_name":"Auth","waves":[{"id":"auth-w3","cluster_name":"Auth","title":"Security pass","description":"desc","actions":[],"prerequisites":["auth-w2"],"delta":{"before":0.65,"after":0.80},"status":"available"}],"reasoning":"needed"}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := ParseNextGenResult(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if result.ClusterName != "Auth" {
		t.Errorf("cluster_name: got %q", result.ClusterName)
	}
	if len(result.Waves) != 1 {
		t.Fatalf("waves: got %d, want 1", len(result.Waves))
	}
	if result.Waves[0].ID != "auth-w3" {
		t.Errorf("wave id: got %q", result.Waves[0].ID)
	}
}

func TestParseNextGenResult_EmptyWaves(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nextgen.json")
	data := `{"cluster_name":"Auth","waves":[],"reasoning":"cluster complete"}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := ParseNextGenResult(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(result.Waves) != 0 {
		t.Errorf("expected 0 waves, got %d", len(result.Waves))
	}
}

func TestParseNextGenResult_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nextgen.json")
	if err := os.WriteFile(path, []byte("{bad json"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := ParseNextGenResult(path)
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "parse nextgen result") {
		t.Errorf("error should contain 'parse nextgen result': %v", err)
	}
}

func TestParseNextGenResult_MissingFile(t *testing.T) {
	_, err := ParseNextGenResult("/nonexistent/file.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}
