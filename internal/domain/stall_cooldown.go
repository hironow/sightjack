package domain

import "time"

// StallCooldown tracks stall escalation emissions and suppresses duplicates
// within a cooldown window. Key = wave_id + error_fingerprint.
type StallCooldown struct {
	window  time.Duration
	emitted map[string]time.Time
	clock   func() time.Time
}

// NewStallCooldown creates a cooldown tracker with the given window duration.
func NewStallCooldown(window time.Duration) *StallCooldown {
	return &StallCooldown{
		window:  window,
		emitted: make(map[string]time.Time),
		clock:   time.Now,
	}
}

// NewStallCooldownWithClock creates a cooldown tracker with a custom clock (for testing).
func NewStallCooldownWithClock(window time.Duration, clock func() time.Time) *StallCooldown {
	return &StallCooldown{
		window:  window,
		emitted: make(map[string]time.Time),
		clock:   clock,
	}
}

// SetClock replaces the clock function (for testing).
func (c *StallCooldown) SetClock(clock func() time.Time) {
	c.clock = clock
}

// Allow returns true if a stall escalation for the given wave+fingerprint
// should be emitted. Returns false if the same combination was emitted
// within the cooldown window.
func (c *StallCooldown) Allow(waveID, fingerprint string) bool {
	key := waveID + ":" + fingerprint
	now := c.clock()
	if last, ok := c.emitted[key]; ok {
		if now.Sub(last) < c.window {
			return false
		}
	}
	c.emitted[key] = now
	return true
}
