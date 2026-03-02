package session_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hironow/sightjack"
	"github.com/hironow/sightjack/internal/session"
)

func TestStatus_EmptyState(t *testing.T) {
	// given — fresh directory with no events, no inbox, no archive
	baseDir := t.TempDir()

	// when
	report := session.Status(baseDir)

	// then
	if report.WavesTotal != 0 {
		t.Errorf("expected WavesTotal=0, got %d", report.WavesTotal)
	}
	if report.InboxCount != 0 {
		t.Errorf("expected InboxCount=0, got %d", report.InboxCount)
	}
	if report.ArchiveCount != 0 {
		t.Errorf("expected ArchiveCount=0, got %d", report.ArchiveCount)
	}
	if report.SuccessRate != 0.0 {
		t.Errorf("expected SuccessRate=0.0, got %f", report.SuccessRate)
	}
	if !report.LastScanned.IsZero() {
		t.Errorf("expected LastScanned to be zero, got %v", report.LastScanned)
	}
}

func TestStatus_WithMailDirs(t *testing.T) {
	// given — create inbox and archive with files
	baseDir := t.TempDir()
	if err := session.EnsureMailDirs(baseDir); err != nil {
		t.Fatal(err)
	}

	inboxDir := sightjack.MailDir(baseDir, sightjack.InboxDir)
	archiveDir := sightjack.MailDir(baseDir, sightjack.ArchiveDir)

	// Create 2 inbox files
	for _, name := range []string{"spec-w1.md", "report-w2.md"} {
		if err := os.WriteFile(filepath.Join(inboxDir, name), []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Create 3 archive files
	for _, name := range []string{"spec-a1.md", "report-a2.md", "feedback-a3.md"} {
		if err := os.WriteFile(filepath.Join(archiveDir, name), []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// when
	report := session.Status(baseDir)

	// then
	if report.InboxCount != 2 {
		t.Errorf("expected InboxCount=2, got %d", report.InboxCount)
	}
	if report.ArchiveCount != 3 {
		t.Errorf("expected ArchiveCount=3, got %d", report.ArchiveCount)
	}
}

func TestStatus_WithEvents(t *testing.T) {
	// given — create event store with waves and scan events
	baseDir := t.TempDir()
	sessionID := "test-session-001"
	store := session.NewEventStore(baseDir, sessionID)

	// Create a scan_completed event with LastScanned
	scanTime := time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)
	scanPayload := sightjack.ScanCompletedPayload{
		Completeness: 0.75,
		ShibitoCount: 2,
		LastScanned:  scanTime,
	}
	scanEvent, err := sightjack.NewEvent(sightjack.EventScanCompleted, scanPayload, scanTime)
	if err != nil {
		t.Fatal(err)
	}
	scanEvent.SessionID = sessionID

	// Create wave_applied event
	appliedPayload := sightjack.WaveAppliedPayload{
		WaveID:      "w1",
		ClusterName: "cluster-a",
		Applied:     3,
		TotalCount:  3,
	}
	appliedEvent, err := sightjack.NewEvent(sightjack.EventWaveApplied, appliedPayload, scanTime.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	appliedEvent.SessionID = sessionID

	// Create wave_rejected event
	rejectedPayload := sightjack.WaveIdentityPayload{
		WaveID:      "w2",
		ClusterName: "cluster-b",
	}
	rejectedEvent, err := sightjack.NewEvent(sightjack.EventWaveRejected, rejectedPayload, scanTime.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	rejectedEvent.SessionID = sessionID

	// Persist events — need to create event store directory first
	eventsDir := filepath.Join(baseDir, sightjack.StateDir, "events", sessionID)
	if err := os.MkdirAll(eventsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := store.Append(scanEvent, appliedEvent, rejectedEvent); err != nil {
		t.Fatal(err)
	}

	// when
	report := session.Status(baseDir)

	// then
	if report.WavesTotal != 2 {
		t.Errorf("expected WavesTotal=2, got %d", report.WavesTotal)
	}
	if report.SuccessRate != 0.5 {
		t.Errorf("expected SuccessRate=0.5, got %f", report.SuccessRate)
	}
	if !report.LastScanned.Equal(scanTime) {
		t.Errorf("expected LastScanned=%v, got %v", scanTime, report.LastScanned)
	}
}

func TestStatus_FormatText(t *testing.T) {
	// given
	report := session.StatusReport{
		LastScanned:  time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC),
		WavesTotal:   12,
		SuccessRate:  0.8,
		InboxCount:   2,
		ArchiveCount: 15,
	}

	// when
	text := report.FormatText()

	// then — verify key lines are present
	expected := []string{
		"sightjack status:",
		"Last scan:",
		"Waves:",
		"Success rate:",
		"Inbox:",
		"Archive:",
	}
	for _, s := range expected {
		if !containsString(text, s) {
			t.Errorf("expected output to contain %q, got:\n%s", s, text)
		}
	}
}

func TestStatus_FormatJSON(t *testing.T) {
	// given
	report := session.StatusReport{
		LastScanned:  time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC),
		WavesTotal:   12,
		SuccessRate:  0.8,
		InboxCount:   2,
		ArchiveCount: 15,
	}

	// when
	data := report.FormatJSON()

	// then — verify it's valid JSON with expected fields
	var parsed map[string]any
	if err := json.Unmarshal([]byte(data), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, data)
	}
	if parsed["waves_total"] != float64(12) {
		t.Errorf("expected waves_total=12, got %v", parsed["waves_total"])
	}
	if parsed["inbox_count"] != float64(2) {
		t.Errorf("expected inbox_count=2, got %v", parsed["inbox_count"])
	}
}

func TestStatus_FormatText_NoEvents(t *testing.T) {
	// given — zero-value report
	report := session.StatusReport{}

	// when
	text := report.FormatText()

	// then
	if !containsString(text, "no scans yet") {
		t.Errorf("expected 'no scans yet' for zero time, got:\n%s", text)
	}
	if !containsString(text, "no events") {
		t.Errorf("expected 'no events' for zero success rate, got:\n%s", text)
	}
}

func containsString(haystack, needle string) bool {
	return len(haystack) > 0 && len(needle) > 0 &&
		// simple substring check
		testContains(haystack, needle)
}

func testContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
