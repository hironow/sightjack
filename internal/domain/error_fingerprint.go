package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// ErrorKind classifies an error as structural or transient.
type ErrorKind string

const (
	// ErrorKindStructural indicates a persistent, non-recoverable error
	// (e.g. missing file, permission denied).
	ErrorKindStructural ErrorKind = "structural"
	// ErrorKindTransient indicates a temporary, potentially self-resolving error
	// (e.g. network timeout, connection reset).
	ErrorKindTransient ErrorKind = "transient"
)

// EventWaveStalled is defined in event.go alongside all other EventType constants.

// structuralPhrases are substrings that indicate structural (non-transient) errors.
var structuralPhrases = []string{
	"permission denied",
	"no such file or directory",
	"not found",
	"access denied",
	"invalid argument",
	"too many open files",
	"read-only file system",
}

// ErrorFingerprint returns a short, stable hash of the error message that can
// be used to detect repeated identical errors across attempts.
func ErrorFingerprint(errMsg string) string {
	h := sha256.New()
	h.Write([]byte(errMsg))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// ClassifyError classifies an error message as structural or transient.
// Structural errors are persistent and require human intervention.
// Transient errors may resolve on retry.
func ClassifyError(errMsg string) ErrorKind {
	lower := strings.ToLower(errMsg)
	for _, phrase := range structuralPhrases {
		if strings.Contains(lower, phrase) {
			return ErrorKindStructural
		}
	}
	return ErrorKindTransient
}

// DetectRepeatedPattern scans a slice of error fingerprints and reports
// whether any single fingerprint appears at least threshold times.
// Returns (true, fingerprint) if a repeated pattern is found, (false, "") otherwise.
func DetectRepeatedPattern(fingerprints []string, threshold int) (bool, string) {
	counts := make(map[string]int, len(fingerprints))
	for _, fp := range fingerprints {
		counts[fp]++
		if counts[fp] >= threshold {
			return true, fp
		}
	}
	return false, ""
}

// WaveStalledPayload is the payload for EventWaveStalled.
type WaveStalledPayload struct { // nosemgrep: domain-primitives.public-string-field-go -- JSON wire format [permanent]
	WaveID      string `json:"wave_id"`
	ClusterName string `json:"cluster_name"`
	Fingerprint string `json:"fingerprint,omitempty"`
	Reason      string `json:"reason"`
}

// MarkStalled sets the wave's status to "stalled" and emits an EventWaveStalled event.
func (a *WaveAggregate) MarkStalled(waveID, clusterName, reason string, opts ...time.Time) (Event, error) { // nosemgrep: domain-primitives.multiple-string-params-go -- waveID/clusterName/reason are distinct semantic roles; typed wrappers deferred [permanent]
	if _, ok := a.findWave(waveID, clusterName); !ok {
		return Event{}, fmt.Errorf("wave %s:%s not found", clusterName, waveID)
	}
	for i := range a.waves {
		if a.waves[i].ID == waveID && a.waves[i].ClusterName == clusterName {
			a.waves[i].Status = "stalled"
			break
		}
	}
	now := time.Now()
	if len(opts) > 0 {
		now = opts[0]
	}
	return NewEvent(EventWaveStalled, WaveStalledPayload{
		WaveID:      waveID,
		ClusterName: clusterName,
		Reason:      reason,
	}, now)
}
