package domain_test

import (
	"testing"

	"github.com/hironow/sightjack/internal/domain"
)

func TestErrorFingerprint_StructuralVsTransient(t *testing.T) {
	// given: a structural error message (missing file, permission denied)
	structuralMsg := "permission denied: cannot write to /etc/config"
	transientMsg := "connection reset by peer"

	// when
	fpStructural := domain.ErrorFingerprint(structuralMsg)
	fpTransient := domain.ErrorFingerprint(transientMsg)

	// then: fingerprints are non-empty strings
	if fpStructural == "" {
		t.Error("ErrorFingerprint: expected non-empty fingerprint for structural error")
	}
	if fpTransient == "" {
		t.Error("ErrorFingerprint: expected non-empty fingerprint for transient error")
	}
	// distinct messages should yield distinct fingerprints
	if fpStructural == fpTransient {
		t.Errorf("ErrorFingerprint: expected distinct fingerprints, both got %q", fpStructural)
	}
}

func TestClassifyError_StructuralVsTransient(t *testing.T) {
	tests := []struct {
		name     string
		errMsg   string
		wantKind domain.ErrorKind
	}{
		{
			name:     "permission denied is structural",
			errMsg:   "open /etc/foo: permission denied",
			wantKind: domain.ErrorKindStructural,
		},
		{
			name:     "no such file is structural",
			errMsg:   "stat /tmp/bar: no such file or directory",
			wantKind: domain.ErrorKindStructural,
		},
		{
			name:     "connection reset is transient",
			errMsg:   "connection reset by peer",
			wantKind: domain.ErrorKindTransient,
		},
		{
			name:     "timeout is transient",
			errMsg:   "context deadline exceeded",
			wantKind: domain.ErrorKindTransient,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			got := domain.ClassifyError(tt.errMsg)

			// then
			if got != tt.wantKind {
				t.Errorf("ClassifyError(%q) = %v, want %v", tt.errMsg, got, tt.wantKind)
			}
		})
	}
}

func TestDetectRepeatedPattern_ReturnsTrue(t *testing.T) {
	// given: a slice of fingerprints with a repeated pattern
	prints := []string{"fp-a", "fp-b", "fp-a", "fp-a"}

	// when
	repeated, fp := domain.DetectRepeatedPattern(prints, 3)

	// then
	if !repeated {
		t.Error("DetectRepeatedPattern: expected repeated=true for 3x occurrence")
	}
	if fp != "fp-a" {
		t.Errorf("DetectRepeatedPattern: expected fp=%q, got %q", "fp-a", fp)
	}
}

func TestDetectRepeatedPattern_ReturnsFalse(t *testing.T) {
	// given: all unique fingerprints
	prints := []string{"fp-a", "fp-b", "fp-c"}

	// when
	repeated, _ := domain.DetectRepeatedPattern(prints, 3)

	// then
	if repeated {
		t.Error("DetectRepeatedPattern: expected repeated=false for all-unique fingerprints")
	}
}

func TestEventWaveStalled_Constant(t *testing.T) {
	// given/when: just access the constant
	ev := domain.EventWaveStalled

	// then: it must be a non-empty EventType
	if ev == "" {
		t.Error("EventWaveStalled: constant must not be empty")
	}
	if string(ev) != "wave.stalled" {
		t.Errorf("EventWaveStalled = %q, want %q", ev, "wave.stalled")
	}
}

func TestMarkStalled_SetsWaveStatus(t *testing.T) {
	// given
	agg := domain.NewWaveAggregate()
	agg.SetWaves([]domain.Wave{
		{ID: "w1", ClusterName: "auth", Status: "available"},
	})

	// when
	_, err := agg.MarkStalled("w1", "auth", "repeated permission-denied pattern")

	// then
	if err != nil {
		t.Fatalf("MarkStalled: unexpected error: %v", err)
	}
	for _, w := range agg.Waves() {
		if w.ID == "w1" && w.ClusterName == "auth" {
			if w.Status != "stalled" {
				t.Errorf("MarkStalled: wave status = %q, want %q", w.Status, "stalled")
			}
		}
	}
}
