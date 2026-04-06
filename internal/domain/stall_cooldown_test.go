package domain_test

import (
	"testing"
	"time"

	"github.com/hironow/sightjack/internal/domain"
)

func TestStallCooldown_AllowsFirstOccurrence(t *testing.T) {
	// given
	cd := domain.NewStallCooldown(30 * time.Minute)

	// when
	allowed := cd.Allow("wave-1", "fp-abc")

	// then
	if !allowed {
		t.Error("expected first occurrence to be allowed")
	}
}

func TestStallCooldown_BlocksDuplicateAfterMarkEmitted(t *testing.T) {
	// given
	cd := domain.NewStallCooldown(30 * time.Minute)
	cd.Allow("wave-1", "fp-abc")
	cd.MarkEmitted("wave-1", "fp-abc")

	// when
	allowed := cd.Allow("wave-1", "fp-abc")

	// then
	if allowed {
		t.Error("expected duplicate within cooldown window to be blocked after MarkEmitted")
	}
}

func TestStallCooldown_AllowRetryWithoutMarkEmitted(t *testing.T) {
	// given: Allow was called but MarkEmitted was NOT (send failed)
	cd := domain.NewStallCooldown(30 * time.Minute)
	cd.Allow("wave-1", "fp-abc")
	// no MarkEmitted — simulates send failure

	// when
	allowed := cd.Allow("wave-1", "fp-abc")

	// then: should be allowed again since emission was never confirmed
	if !allowed {
		t.Error("expected re-Allow after failed send (no MarkEmitted)")
	}
}

func TestStallCooldown_AllowsDifferentFingerprint(t *testing.T) {
	// given
	cd := domain.NewStallCooldown(30 * time.Minute)
	cd.Allow("wave-1", "fp-abc")
	cd.MarkEmitted("wave-1", "fp-abc")

	// when
	allowed := cd.Allow("wave-1", "fp-xyz")

	// then
	if !allowed {
		t.Error("expected different fingerprint to be allowed")
	}
}

func TestStallCooldown_AllowsAfterWindowExpires(t *testing.T) {
	// given
	cd := domain.NewStallCooldownWithClock(30*time.Minute, func() time.Time {
		return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	})
	cd.Allow("wave-1", "fp-abc")
	cd.MarkEmitted("wave-1", "fp-abc")

	// advance clock past cooldown
	cd.SetClock(func() time.Time {
		return time.Date(2026, 1, 1, 0, 31, 0, 0, time.UTC)
	})

	// when
	allowed := cd.Allow("wave-1", "fp-abc")

	// then
	if !allowed {
		t.Error("expected allow after cooldown window expires")
	}
}
