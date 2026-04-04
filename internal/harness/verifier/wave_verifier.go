package verifier

import (
	"fmt"

	"github.com/hironow/sightjack/internal/domain"
)

// ValidateWaveApplyResult checks the apply result for degenerate or invalid states.
// Returns an error if the result is nil, empty when actions were expected,
// or reports more applied actions than expected.
func ValidateWaveApplyResult(result *domain.WaveApplyResult, expectedActions int) error {
	if result == nil {
		return fmt.Errorf("wave apply result is nil")
	}
	if expectedActions > 0 && result.Applied == 0 && result.TotalCount == 0 && len(result.Errors) == 0 {
		return fmt.Errorf("wave apply result is empty (expected %d actions)", expectedActions)
	}
	if result.Applied > expectedActions {
		return fmt.Errorf("wave apply result reports %d applied but only %d actions expected", result.Applied, expectedActions)
	}
	return nil
}

// ValidateWavePrerequisites removes prerequisites referencing waves not in the wave set.
// Returns the cleaned wave list and the count of removed dangling prerequisites.
func ValidateWavePrerequisites(waves []domain.Wave) ([]domain.Wave, int) {
	allKeys := make(map[string]bool, len(waves))
	for _, w := range waves {
		allKeys[domain.WaveKey(w)] = true
	}
	result := make([]domain.Wave, len(waves))
	copy(result, waves)
	var removed int
	for i, w := range result {
		var clean []string
		for _, p := range w.Prerequisites {
			if allKeys[p] {
				clean = append(clean, p)
			} else {
				removed++
			}
		}
		result[i].Prerequisites = clean
	}
	return result, removed
}

