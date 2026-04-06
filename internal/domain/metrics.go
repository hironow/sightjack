package domain

import (
	"fmt"
)

// FormatSuccessRate returns a human-readable string for the given success rate.
// Returns "no events" when total is zero.
func FormatSuccessRate(rate float64, success, total int) string {
	if total == 0 {
		return "no events"
	}
	return fmt.Sprintf("%.1f%% (%d/%d)", rate*100, success, total)
}

// SuccessRate calculates the wave success rate from a list of events.
// It counts EventWaveAppliedV2 as success and EventWaveRejectedV2 as failure.
// Returns 0.0 if there are no relevant events.
func SuccessRate(events []Event) float64 {
	var success, total int
	for _, ev := range events {
		switch ev.Type {
		case EventWaveApplied, EventWaveAppliedV2:
			success++
			total++
		case EventWaveRejected, EventWaveRejectedV2:
			total++
		}
	}
	if total == 0 {
		return 0.0
	}
	return float64(success) / float64(total)
}
