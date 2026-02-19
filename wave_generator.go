package sightjack

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// maxWavesPerCluster is the cap on total waves per cluster.
// Beyond this count, nextgen is skipped to prevent infinite wave growth.
const maxWavesPerCluster = 8

// NeedsMoreWaves returns true when post-completion wave generation should run
// for the given cluster. It returns false (skip nextgen) when any of:
//   - cluster completeness >= 0.95 (effectively done)
//   - available (non-completed) waves still remain for the cluster
//   - total wave count for the cluster >= maxWavesPerCluster
func NeedsMoreWaves(cluster ClusterScanResult, waves []Wave) bool {
	if cluster.Completeness >= 0.95 {
		return false
	}
	var clusterTotal int
	hasAvailable := false
	for _, w := range waves {
		if w.ClusterName != cluster.Name {
			continue
		}
		clusterTotal++
		if w.Status == "available" || w.Status == "locked" {
			hasAvailable = true
		}
	}
	if hasAvailable {
		return false
	}
	if clusterTotal >= maxWavesPerCluster {
		return false
	}
	return true
}

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

// GenerateNextWavesDryRun saves the nextgen prompt to a file instead of executing Claude.
func GenerateNextWavesDryRun(cfg *Config, scanDir string, completedWave Wave, cluster ClusterScanResult, completedWaves []Wave, existingADRs []ExistingADR, rejectedActions []WaveAction, strictness string) error {
	prompt, err := buildNextGenPrompt(cfg, scanDir, completedWave, cluster, completedWaves, existingADRs, rejectedActions, strictness)
	if err != nil {
		return err
	}
	dryRunName := fmt.Sprintf("nextgen_%s_%s", sanitizeName(completedWave.ClusterName), sanitizeName(completedWave.ID))
	return RunClaudeDryRun(cfg, prompt, scanDir, dryRunName)
}

// GenerateNextWaves executes post-completion wave generation for a cluster.
func GenerateNextWaves(ctx context.Context, cfg *Config, scanDir string, completedWave Wave, cluster ClusterScanResult, completedWaves []Wave, existingADRs []ExistingADR, rejectedActions []WaveAction, strictness string) ([]Wave, error) {
	clearNextgenOutput(scanDir, completedWave)
	outputFile := filepath.Join(scanDir, nextgenFileName(completedWave))

	prompt, err := buildNextGenPrompt(cfg, scanDir, completedWave, cluster, completedWaves, existingADRs, rejectedActions, strictness)
	if err != nil {
		return nil, err
	}

	LogScan("Generating next waves: %s", completedWave.ClusterName)
	if _, err := RunClaude(ctx, cfg, prompt, io.Discard); err != nil {
		return nil, fmt.Errorf("nextgen %s: %w", completedWave.ClusterName, err)
	}

	result, err := ParseNextGenResult(outputFile)
	if err != nil {
		return nil, fmt.Errorf("parse nextgen %s: %w", completedWave.ClusterName, err)
	}

	newWaves := NormalizeWavePrerequisites(result.Waves)
	if len(newWaves) > 0 {
		LogOK("Generated %d new wave(s) for %s: %s", len(newWaves), completedWave.ClusterName, result.Reasoning)
	}
	return newWaves, nil
}

// buildNextGenPrompt constructs the prompt for post-completion wave generation.
func buildNextGenPrompt(cfg *Config, scanDir string, completedWave Wave, cluster ClusterScanResult, completedWaves []Wave, existingADRs []ExistingADR, rejectedActions []WaveAction, strictness string) (string, error) {
	outputFile := filepath.Join(scanDir, nextgenFileName(completedWave))

	issuesJSON, err := json.Marshal(cluster.Issues)
	if err != nil {
		return "", fmt.Errorf("marshal issues: %w", err)
	}

	completedJSON, err := json.Marshal(completedWaves)
	if err != nil {
		return "", fmt.Errorf("marshal completed waves: %w", err)
	}

	var rejectedStr string
	if len(rejectedActions) > 0 {
		rejectedJSON, err := json.Marshal(rejectedActions)
		if err != nil {
			return "", fmt.Errorf("marshal rejected actions: %w", err)
		}
		rejectedStr = string(rejectedJSON)
	}

	dodSection := ResolveDoDSection(cfg.DoDTemplates, completedWave.ClusterName)

	return RenderNextGenPrompt(cfg.Lang, NextGenPromptData{
		ClusterName:     completedWave.ClusterName,
		Completeness:    fmt.Sprintf("%.0f", cluster.Completeness*100),
		Issues:          string(issuesJSON),
		CompletedWaves:  string(completedJSON),
		ExistingADRs:    existingADRs,
		RejectedActions: rejectedStr,
		DoDSection:      dodSection,
		OutputPath:      outputFile,
		StrictnessLevel: strictness,
	})
}
