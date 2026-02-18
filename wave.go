package sightjack

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ParseWaveGenerateResult reads and parses a wave_{name}.json output file.
func ParseWaveGenerateResult(path string) (*WaveGenerateResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read wave result: %w", err)
	}
	var result WaveGenerateResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse wave result: %w", err)
	}
	return &result, nil
}

// ParseWaveApplyResult reads and parses an apply_{wave_id}.json output file.
func ParseWaveApplyResult(path string) (*WaveApplyResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read apply result: %w", err)
	}
	var result WaveApplyResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse apply result: %w", err)
	}
	return &result, nil
}

// WaveKey returns a globally unique key for a wave: "ClusterName:ID".
func WaveKey(w Wave) string {
	return w.ClusterName + ":" + w.ID
}

// NormalizeWavePrerequisites prefixes bare prerequisite IDs with the wave's own
// cluster name so that all keys in the completed map use the composite format.
// Prerequisites that already contain ":" are left unchanged.
func NormalizeWavePrerequisites(waves []Wave) []Wave {
	result := make([]Wave, len(waves))
	copy(result, waves)
	for i, w := range result {
		normalized := make([]string, len(w.Prerequisites))
		for j, p := range w.Prerequisites {
			if strings.Contains(p, ":") {
				normalized[j] = p
			} else {
				normalized[j] = w.ClusterName + ":" + p
			}
		}
		result[i].Prerequisites = normalized
	}
	return result
}

// MergeWaveResults flattens multiple per-cluster wave results into a single wave list,
// normalizing prerequisite IDs to the composite "ClusterName:ID" format.
func MergeWaveResults(results []WaveGenerateResult) []Wave {
	var all []Wave
	for _, r := range results {
		all = append(all, r.Waves...)
	}
	return NormalizeWavePrerequisites(all)
}

// AvailableWaves returns waves that have "available" status and are not completed.
// The completed map is keyed by WaveKey (ClusterName:ID).
func AvailableWaves(waves []Wave, completed map[string]bool) []Wave {
	var available []Wave
	for _, w := range waves {
		if w.Status == "available" && !completed[WaveKey(w)] {
			available = append(available, w)
		}
	}
	return available
}

// waveApplyFileName returns the output filename for a wave apply result.
// Includes cluster name to avoid collisions when wave IDs are duplicated across clusters.
func waveApplyFileName(wave Wave) string {
	return fmt.Sprintf("apply_%s_%s.json", sanitizeName(wave.ClusterName), sanitizeName(wave.ID))
}

// RunWaveApply executes Pass 4: apply a single approved wave via Claude Code.
func RunWaveApply(ctx context.Context, cfg *Config, scanDir string, wave Wave) (*WaveApplyResult, error) {
	applyFile := filepath.Join(scanDir, waveApplyFileName(wave))

	actionsJSON, err := json.Marshal(wave.Actions)
	if err != nil {
		return nil, fmt.Errorf("marshal wave actions: %w", err)
	}

	prompt, err := RenderWaveApplyPrompt(cfg.Lang, WaveApplyPromptData{
		WaveID:          wave.ID,
		ClusterName:     wave.ClusterName,
		Title:           wave.Title,
		Actions:         string(actionsJSON),
		OutputPath:      applyFile,
		StrictnessLevel: string(cfg.Strictness.Default),
		LabelsEnabled:   cfg.Labels.Enabled,
		LabelPrefix:     cfg.Labels.Prefix,
	})
	if err != nil {
		return nil, fmt.Errorf("render apply prompt: %w", err)
	}

	LogScan("Applying wave: %s - %s", wave.ClusterName, wave.Title)
	if _, err := RunClaude(ctx, cfg, prompt, os.Stdout); err != nil {
		return nil, fmt.Errorf("wave apply %s: %w", wave.ID, err)
	}

	result, err := ParseWaveApplyResult(applyFile)
	if err != nil {
		return nil, fmt.Errorf("parse apply result %s: %w", wave.ID, err)
	}

	LogOK("Wave %s applied: %d actions", wave.ID, result.Applied)
	return result, nil
}

// EvaluateUnlocks checks locked waves and unlocks them if all prerequisites are met.
// Prerequisites and the completed map both use the composite "ClusterName:ID" format.
func EvaluateUnlocks(waves []Wave, completed map[string]bool) []Wave {
	result := make([]Wave, len(waves))
	copy(result, waves)
	for i, w := range result {
		if w.Status != "locked" {
			continue
		}
		allMet := true
		for _, prereq := range w.Prerequisites {
			if !completed[prereq] {
				allMet = false
				break
			}
		}
		if allMet {
			result[i].Status = "available"
		}
	}
	return result
}
