package domain_test

import (
	"testing"
	"time"

	"github.com/hironow/sightjack/internal/domain"
)

func TestProjector_SerializeDeserialize_RoundTrip(t *testing.T) {
	// given: projector with accumulated state
	projector := domain.NewProjector()
	events := []domain.Event{
		makeProjectorEvent(domain.EventSessionStarted, domain.SessionStartedPayload{
			Project: "test-project", StrictnessLevel: "strict",
		}),
		makeProjectorEvent(domain.EventScanCompleted, domain.ScanCompletedPayload{
			Completeness: 0.75,
			ShibitoCount: 3,
			Clusters: []domain.ClusterState{
				{Name: "core", Completeness: 0.8, IssueCount: 5},
			},
		}),
		makeProjectorEvent(domain.EventWavesGenerated, domain.WavesGeneratedPayload{
			Waves: []domain.WaveState{
				{ID: "w1", ClusterName: "core", Status: "available"},
			},
		}),
		makeProjectorEvent(domain.EventADRGenerated, domain.ADRGeneratedPayload{ADRID: "ADR-001"}),
		makeProjectorEvent(domain.EventFeedbackSent, nil),
	}
	projector.Rebuild(events)

	// when: serialize then deserialize
	data, err := projector.Serialize()
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	restored := domain.NewProjector()
	if err := restored.Deserialize(data); err != nil {
		t.Fatalf("deserialize: %v", err)
	}

	// then: states match
	orig := projector.State()
	got := restored.State()
	if got.Project != orig.Project {
		t.Errorf("Project: got %q, want %q", got.Project, orig.Project)
	}
	if got.Completeness != orig.Completeness {
		t.Errorf("Completeness: got %f, want %f", got.Completeness, orig.Completeness)
	}
	if len(got.Waves) != len(orig.Waves) {
		t.Errorf("Waves: got %d, want %d", len(got.Waves), len(orig.Waves))
	}
	if got.ADRCount != orig.ADRCount {
		t.Errorf("ADRCount: got %d, want %d", got.ADRCount, orig.ADRCount)
	}
	if got.FeedbackCount != orig.FeedbackCount {
		t.Errorf("FeedbackCount: got %d, want %d", got.FeedbackCount, orig.FeedbackCount)
	}
}

func TestProjector_DeserializeCorrupt(t *testing.T) {
	// given
	projector := domain.NewProjector()

	// when
	err := projector.Deserialize([]byte("not-json"))

	// then
	if err == nil {
		t.Fatal("expected error for corrupt data")
	}
}

func TestProjector_SnapshotPlusDelta_DedupPreserved(t *testing.T) {
	// given: build state from base events, serialize
	baseEvents := []domain.Event{
		makeProjectorEvent(domain.EventSessionStarted, domain.SessionStartedPayload{Project: "proj"}),
		makeProjectorEvent(domain.EventWavesGenerated, domain.WavesGeneratedPayload{
			Waves: []domain.WaveState{{ID: "w1", ClusterName: "core", Status: "available"}},
		}),
		makeProjectorEvent(domain.EventADRGenerated, domain.ADRGeneratedPayload{ADRID: "ADR-001"}),
	}
	deltaEvents := []domain.Event{
		// Duplicate wave and ADR — should be deduplicated
		makeProjectorEvent(domain.EventWavesGenerated, domain.WavesGeneratedPayload{
			Waves: []domain.WaveState{{ID: "w1", ClusterName: "core", Status: "available"}},
		}),
		makeProjectorEvent(domain.EventADRGenerated, domain.ADRGeneratedPayload{ADRID: "ADR-001"}),
		// New wave and ADR
		makeProjectorEvent(domain.EventWavesGenerated, domain.WavesGeneratedPayload{
			Waves: []domain.WaveState{{ID: "w2", ClusterName: "core", Status: "locked"}},
		}),
		makeProjectorEvent(domain.EventADRGenerated, domain.ADRGeneratedPayload{ADRID: "ADR-002"}),
		makeProjectorEvent(domain.EventFeedbackSent, nil),
	}

	// Full replay for comparison
	fullProjector := domain.NewProjector()
	fullProjector.Rebuild(append(baseEvents, deltaEvents...))
	expected := fullProjector.State()

	// Snapshot + delta
	snapProjector := domain.NewProjector()
	snapProjector.Rebuild(baseEvents)
	snapData, _ := snapProjector.Serialize()

	restored := domain.NewProjector()
	restored.Deserialize(snapData)
	for _, ev := range deltaEvents {
		restored.Apply(ev)
	}
	got := restored.State()

	// then: snapshot+delta == full replay (dedup maps preserved)
	if len(got.Waves) != len(expected.Waves) {
		t.Errorf("Waves: got %d, want %d", len(got.Waves), len(expected.Waves))
	}
	if got.ADRCount != expected.ADRCount {
		t.Errorf("ADRCount: got %d, want %d (dedup broken if >2)", got.ADRCount, expected.ADRCount)
	}
	if got.FeedbackCount != expected.FeedbackCount {
		t.Errorf("FeedbackCount: got %d, want %d", got.FeedbackCount, expected.FeedbackCount)
	}
}

func TestProjector_BackwardCompatible_WithProjectState(t *testing.T) {
	// given: same events
	events := []domain.Event{
		makeProjectorEvent(domain.EventSessionStarted, domain.SessionStartedPayload{Project: "test"}),
		makeProjectorEvent(domain.EventScanCompleted, domain.ScanCompletedPayload{Completeness: 0.5}),
		makeProjectorEvent(domain.EventFeedbackSent, nil),
	}

	// when
	projectorState := domain.NewProjector()
	projectorState.Rebuild(events)
	legacy := domain.ProjectState(events)

	// then: both produce same result
	got := projectorState.State()
	if got.Project != legacy.Project {
		t.Errorf("Project: projector=%q, legacy=%q", got.Project, legacy.Project)
	}
	if got.Completeness != legacy.Completeness {
		t.Errorf("Completeness: projector=%f, legacy=%f", got.Completeness, legacy.Completeness)
	}
	if got.FeedbackCount != legacy.FeedbackCount {
		t.Errorf("FeedbackCount: projector=%d, legacy=%d", got.FeedbackCount, legacy.FeedbackCount)
	}
}

func makeProjectorEvent(eventType domain.EventType, data any) domain.Event { //nolint:unparam
	ev, err := domain.NewEvent(eventType, data, time.Now())
	if err != nil {
		panic(err)
	}
	return ev
}
