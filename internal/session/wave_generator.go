package session

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	sightjack "github.com/hironow/sightjack"
	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/platform"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// maxWavesPerCluster is the cap on total waves per cluster.
// Beyond this count, nextgen is skipped to prevent infinite wave growth.
const maxWavesPerCluster = 8

// NeedsMoreWaves returns true when post-completion wave generation should run
// for the given cluster. It returns false (skip nextgen) when any of:
//   - cluster completeness >= 0.95 (effectively done)
//   - available (non-completed) waves still remain for the cluster
//   - total wave count for the cluster >= maxWavesPerCluster
func NeedsMoreWaves(cluster sightjack.ClusterScanResult, waves []sightjack.Wave) bool {
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
		if w.Status == "available" || w.Status == "locked" || w.Status == "partial" {
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

// NextgenFileName returns the output filename for a nextgen wave generation run.
func NextgenFileName(wave sightjack.Wave) string {
	return fmt.Sprintf("nextgen_%s_%s.json", domain.SanitizeName(wave.ClusterName), domain.SanitizeName(wave.ID))
}

// ClearNextgenOutput removes any existing nextgen output file.
func ClearNextgenOutput(scanDir string, wave sightjack.Wave) {
	path := filepath.Join(scanDir, NextgenFileName(wave))
	_ = os.Remove(path)
}

// ParseNextGenResult reads and parses a nextgen wave generation result JSON file.
func ParseNextGenResult(path string) (*sightjack.NextGenResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read nextgen result: %w", err)
	}
	var result sightjack.NextGenResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse nextgen result: %w", err)
	}
	return &result, nil
}

// GenerateNextWavesDryRun saves the nextgen prompt to a file instead of executing Claude.
func GenerateNextWavesDryRun(cfg *sightjack.Config, scanDir string, completedWave sightjack.Wave, cluster sightjack.ClusterScanResult, completedWaves []sightjack.Wave, existingADRs []sightjack.ExistingADR, rejectedActions []sightjack.WaveAction, strictness string, feedback []*DMail, reports []*DMail, logger *domain.Logger) error {
	prompt, err := BuildNextGenPrompt(cfg, scanDir, completedWave, cluster, completedWaves, existingADRs, rejectedActions, strictness, feedback, reports)
	if err != nil {
		return err
	}
	dryRunName := fmt.Sprintf("nextgen_%s_%s", domain.SanitizeName(completedWave.ClusterName), domain.SanitizeName(completedWave.ID))
	return RunClaudeDryRun(cfg, prompt, scanDir, dryRunName, logger)
}

// GenerateNextWaves executes post-completion wave generation for a cluster.
func GenerateNextWaves(ctx context.Context, cfg *sightjack.Config, scanDir string, completedWave sightjack.Wave, cluster sightjack.ClusterScanResult, completedWaves []sightjack.Wave, existingADRs []sightjack.ExistingADR, rejectedActions []sightjack.WaveAction, strictness string, feedback []*DMail, reports []*DMail, logger *domain.Logger) ([]sightjack.Wave, error) {
	ctx, nextgenSpan := platform.Tracer.Start(ctx, "wave.nextgen",
		trace.WithAttributes(
			attribute.String("wave.cluster_name", completedWave.ClusterName),
		),
	)
	defer nextgenSpan.End()

	ClearNextgenOutput(scanDir, completedWave)
	outputFile := filepath.Join(scanDir, NextgenFileName(completedWave))

	prompt, err := BuildNextGenPrompt(cfg, scanDir, completedWave, cluster, completedWaves, existingADRs, rejectedActions, strictness, feedback, reports)
	if err != nil {
		return nil, err
	}

	// Save prompt + tee output for debugging.
	promptBase := strings.TrimSuffix(NextgenFileName(completedWave), ".json")
	if err := os.WriteFile(filepath.Join(scanDir, promptBase+"_prompt.md"), []byte(prompt), 0644); err != nil {
		logger.Warn("save nextgen prompt: %v", err)
	}
	nextgenLog, nextgenLogErr := os.Create(filepath.Join(scanDir, promptBase+"_output.log"))
	nextgenOut := io.Writer(io.Discard)
	if nextgenLogErr == nil {
		defer nextgenLog.Close()
		nextgenOut = nextgenLog
	} else {
		logger.Warn("create nextgen log: %v", nextgenLogErr)
	}

	logger.Info("Generating next waves: %s", completedWave.ClusterName)
	if _, err := RunClaude(ctx, cfg, prompt, nextgenOut, logger, WithAllowedTools(slices.Concat(BaseAllowedTools, GHAllowedTools, LinearMCPAllowedTools)...)); err != nil {
		return nil, fmt.Errorf("nextgen %s: %w", completedWave.ClusterName, err)
	}

	if normErr := NormalizeJSONFile(outputFile); normErr != nil {
		logger.Warn("normalize nextgen JSON: %v", normErr)
	}
	result, err := ParseNextGenResult(outputFile)
	if err != nil {
		return nil, fmt.Errorf("parse nextgen %s: %w", completedWave.ClusterName, err)
	}

	newWaves := domain.NormalizeWavePrerequisites(result.Waves)
	if len(newWaves) > 0 {
		logger.OK("Generated %d new wave(s) for %s: %s", len(newWaves), completedWave.ClusterName, result.Reasoning)
	}
	return newWaves, nil
}

// BuildNextGenPrompt constructs the prompt for post-completion wave generation.
func BuildNextGenPrompt(cfg *sightjack.Config, scanDir string, completedWave sightjack.Wave, cluster sightjack.ClusterScanResult, completedWaves []sightjack.Wave, existingADRs []sightjack.ExistingADR, rejectedActions []sightjack.WaveAction, strictness string, feedback []*DMail, reports []*DMail) (string, error) {
	outputFile := filepath.Join(scanDir, NextgenFileName(completedWave))

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

	dodSection := sightjack.ResolveDoDSection(cfg.DoDTemplates, completedWave.ClusterName)

	return sightjack.RenderNextGenPrompt(cfg.Lang, sightjack.NextGenPromptData{
		ClusterName:     completedWave.ClusterName,
		Completeness:    fmt.Sprintf("%.0f", cluster.Completeness*100),
		Issues:          string(issuesJSON),
		CompletedWaves:  string(completedJSON),
		ExistingADRs:    existingADRs,
		RejectedActions: rejectedStr,
		FeedbackSection: FormatFeedbackForPrompt(feedback),
		ReportSection:   FormatReportsForPrompt(reports),
		DoDSection:      dodSection,
		OutputPath:      outputFile,
		StrictnessLevel: strictness,
	})
}
