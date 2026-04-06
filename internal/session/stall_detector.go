package session

import (
	"context"
	"time"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/usecase/port"
)

// StallDetector tracks repeated structural failures and emits stall-escalation
// D-Mails when the threshold is reached. Integrates ApplyErrorBudget,
// ErrorFingerprint, and StallCooldown.
type StallDetector struct {
	threshold int // N consecutive structural failures to trigger stall
	cooldown  *domain.StallCooldown
	// fingerprint tracking per wave
	waveFingerprints map[string][]string // waveKey → list of error fingerprints
	logger           domain.Logger
}

// NewStallDetector creates a stall detector with the given threshold and cooldown window.
func NewStallDetector(threshold int, cooldownWindow time.Duration, logger domain.Logger) *StallDetector {
	return &StallDetector{
		threshold:        threshold,
		cooldown:         domain.NewStallCooldown(cooldownWindow),
		waveFingerprints: make(map[string][]string),
		logger:           logger,
	}
}

// StallResult holds stall detection outcome for the caller to emit events.
type StallResult struct {
	Detected    bool
	WaveID      string
	ClusterName string
	Fingerprint string
	Reason      string
}

// RecordFailure records a partial failure for a wave and checks if stall threshold is met.
// If threshold is met and cooldown allows, emits a stall-escalation D-Mail.
// Returns a StallResult so the caller can emit the corresponding domain event.
func (d *StallDetector) RecordFailure(ctx context.Context, store port.OutboxStore, wave domain.Wave, errors []string) StallResult {
	waveKey := domain.WaveKey(wave)

	// Collect fingerprints from structural errors
	structural := domain.StructuralErrors(errors)
	for _, e := range structural {
		fp := domain.ErrorFingerprint(e)
		d.waveFingerprints[waveKey] = append(d.waveFingerprints[waveKey], fp)
	}

	// Check for repeated pattern
	fps := d.waveFingerprints[waveKey]
	detected, fingerprint := domain.DetectRepeatedPattern(fps, d.threshold)
	if !detected {
		return StallResult{}
	}

	// Check cooldown (keyed by cluster:waveID, not waveID alone)
	if !d.cooldown.Allow(waveKey, fingerprint) {
		d.logger.Debug("Stall escalation suppressed (cooldown): wave=%s fp=%s", waveKey, fingerprint)
		return StallResult{}
	}

	// Count occurrences of the detected fingerprint specifically
	fpCount := 0
	for _, f := range fps {
		if f == fingerprint {
			fpCount++
		}
	}

	reason := "repeated structural error detected"
	d.logger.Warn("Stall detected for wave %s (fingerprint=%s, count=%d). Emitting escalation D-Mail.", waveKey, fingerprint, fpCount)

	if err := ComposeStallEscalation(ctx, store, wave, structural, reason, fingerprint, fpCount); err != nil {
		d.logger.Error("Failed to compose stall escalation D-Mail: %v", err)
		return StallResult{}
	}

	// Mark cooldown only after successful send to allow retry on failure.
	d.cooldown.MarkEmitted(waveKey, fingerprint)

	return StallResult{
		Detected:    true,
		WaveID:      wave.ID,
		ClusterName: wave.ClusterName,
		Fingerprint: fingerprint,
		Reason:      reason,
	}
}

// RecordSuccess resets the fingerprint tracking for a wave (successful apply clears stall state).
func (d *StallDetector) RecordSuccess(wave domain.Wave) {
	delete(d.waveFingerprints, domain.WaveKey(wave))
}
