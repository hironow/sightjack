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

// Allow checks whether a stall escalation for the given wave+fingerprint
// should be emitted. Returns false if the same combination was emitted
// within the cooldown window. Does NOT record the emission — call
// MarkEmitted after the D-Mail is successfully sent.
func (c *StallCooldown) Allow(waveID, fingerprint string) bool {
	key := waveID + ":" + fingerprint
	now := c.clock()
	if last, ok := c.emitted[key]; ok {
		if now.Sub(last) < c.window {
			return false
		}
	}
	return true
}

// MarkEmitted records that a stall escalation was successfully emitted.
// Call this after the D-Mail send succeeds to start the cooldown window.
func (c *StallCooldown) MarkEmitted(waveID, fingerprint string) {
	key := waveID + ":" + fingerprint
	c.emitted[key] = c.clock()
}
