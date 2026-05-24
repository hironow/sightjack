package session

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/hironow/sightjack/internal/domain"
)

// ParseClassifyResult reads and parses the classify.json output file.
func ParseClassifyResult(path string) (*domain.ClassifyResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read classify result: %w", err)
	}
	var result domain.ClassifyResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse classify result: %w", err)
	}
	return &result, nil
}

// ParseClusterScanResult reads and parses a cluster_{name}.json output file.
// Used by the MCP data-plane (sightjack.get_scan_result) to surface aggregated
// cluster info from the session's scan dir.
func ParseClusterScanResult(path string) (*domain.ClusterScanResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read cluster result: %w", err)
	}
	var result domain.ClusterScanResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse cluster result: %w", err)
	}
	return &result, nil
}
