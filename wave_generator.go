package sightjack

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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

// GenerateNextWavesDryRun saves the nextgen prompt to a file instead of executing Claude.
func GenerateNextWavesDryRun(cfg *Config, scanDir string, completedWave Wave, cluster ClusterScanResult, completedWaves []Wave, existingADRs []ExistingADR, rejectedActions []WaveAction) error {
	prompt, err := buildNextGenPrompt(cfg, scanDir, completedWave, cluster, completedWaves, existingADRs, rejectedActions)
	if err != nil {
		return err
	}
	dryRunName := fmt.Sprintf("nextgen_%s_%s", sanitizeName(completedWave.ClusterName), sanitizeName(completedWave.ID))
	return RunClaudeDryRun(cfg, prompt, scanDir, dryRunName)
}

// GenerateNextWaves executes post-completion wave generation for a cluster.
func GenerateNextWaves(ctx context.Context, cfg *Config, scanDir string, completedWave Wave, cluster ClusterScanResult, completedWaves []Wave, existingADRs []ExistingADR, rejectedActions []WaveAction) ([]Wave, error) {
	clearNextgenOutput(scanDir, completedWave)
	outputFile := filepath.Join(scanDir, nextgenFileName(completedWave))

	prompt, err := buildNextGenPrompt(cfg, scanDir, completedWave, cluster, completedWaves, existingADRs, rejectedActions)
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
func buildNextGenPrompt(cfg *Config, scanDir string, completedWave Wave, cluster ClusterScanResult, completedWaves []Wave, existingADRs []ExistingADR, rejectedActions []WaveAction) (string, error) {
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

	var dodSection string
	if cfg.DoDTemplates != nil {
		if matched, key := MatchDoDTemplate(cfg.DoDTemplates, completedWave.ClusterName); matched {
			dodSection = FormatDoDSection(cfg.DoDTemplates[key])
		}
	}

	return RenderNextGenPrompt(cfg.Lang, NextGenPromptData{
		ClusterName:     completedWave.ClusterName,
		Completeness:    fmt.Sprintf("%.0f", cluster.Completeness*100),
		Issues:          string(issuesJSON),
		CompletedWaves:  string(completedJSON),
		ExistingADRs:    existingADRs,
		RejectedActions: rejectedStr,
		DoDSection:      dodSection,
		OutputPath:      outputFile,
		StrictnessLevel: string(cfg.Strictness.Default),
	})
}
