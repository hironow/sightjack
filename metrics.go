package sightjack

// SuccessRate calculates the wave success rate from a list of events.
// It counts EventWaveApplied as success and EventWaveRejected as failure.
// Returns 0.0 if there are no relevant events.
func SuccessRate(events []Event) float64 {
	var success, total int
	for _, ev := range events {
		switch ev.Type {
		case EventWaveApplied:
			success++
			total++
		case EventWaveRejected:
			total++
		}
	}
	if total == 0 {
		return 0.0
	}
	return float64(success) / float64(total)
}
