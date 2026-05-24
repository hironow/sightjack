package session

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/hironow/sightjack/internal/domain"
)

// ParseWaveGenerateResult reads and parses a wave_{name}.json output file.
// Used by the MCP data-plane (sightjack.next_wave) to surface available waves
// from the session's scan dir.
func ParseWaveGenerateResult(path string) (*domain.WaveGenerateResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read wave result: %w", err)
	}
	var result domain.WaveGenerateResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse wave result: %w", err)
	}
	return &result, nil
}
