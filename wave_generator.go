package sightjack

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// nextgenFileName returns the output filename for a nextgen wave generation run.
func nextgenFileName(wave Wave) string {
	return fmt.Sprintf("nextgen_%s_%s.json", sanitizeName(wave.ClusterName), sanitizeName(wave.ID))
}

// clearNextgenOutput removes any existing nextgen output file.
func clearNextgenOutput(scanDir string, wave Wave) {
	path := filepath.Join(scanDir, nextgenFileName(wave))
	os.Remove(path)
}

// ParseNextGenResult reads and parses a nextgen wave generation result JSON file.
func ParseNextGenResult(path string) (*NextGenResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read nextgen result: %w", err)
	}
	var result NextGenResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse nextgen result: %w", err)
	}
	return &result, nil
}
