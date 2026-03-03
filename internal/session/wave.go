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

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/platform"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// ParseWaveGenerateResult reads and parses a wave_{name}.json output file.
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

// ParseWaveApplyResult reads and parses an apply_{wave_id}.json output file.
func ParseWaveApplyResult(path string) (*domain.WaveApplyResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read apply result: %w", err)
	}
	var result domain.WaveApplyResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse apply result: %w", err)
	}
	return &result, nil
}

// ToApplyResult converts the internal domain.WaveApplyResult to the pipe wire format domain.ApplyResult.
// It builds per-action results from the wave's actions and the internal result's error list.
func ToApplyResult(wave domain.Wave, internal *domain.WaveApplyResult) domain.ApplyResult {
	actions := make([]domain.ActionResult, 0, len(wave.Actions))

	// Build per-action results: first N actions succeed (N = Applied),
	// remaining get error messages from the Errors list.
	for i, a := range wave.Actions {
		ar := domain.ActionResult{
			Type:    a.Type,
			IssueID: a.IssueID,
			Success: i < internal.Applied,
		}
		if !ar.Success {
			errIdx := i - internal.Applied
			if errIdx >= 0 && errIdx < len(internal.Errors) {
				ar.Error = internal.Errors[errIdx]
			} else {
				ar.Error = "unknown error"
			}
		}
		actions = append(actions, ar)
	}

	// Interpolate completeness based on the ratio of successfully applied actions.
	total := len(wave.Actions)
	var completeness float64
	if total == 0 {
		completeness = wave.Delta.Before
	} else if internal.Applied < total {
		ratio := float64(internal.Applied) / float64(total)
		completeness = wave.Delta.Before + (wave.Delta.After-wave.Delta.Before)*ratio
	} else {
		completeness = wave.Delta.After
	}

	if total == 0 || internal.Applied >= total {
		wave.Status = "completed"
	} else {
		wave.Status = "partial"
	}

	return domain.ApplyResult{
		WaveID:          internal.WaveID,
		AppliedActions:  actions,
		RippleEffects:   internal.Ripples,
		NewCompleteness: completeness,
		CompletedWave:   &wave,
	}
}

// --- Domain wrapper functions (cmd → session → domain) ---

// WaveKey returns a globally unique key for a wave: "ClusterName:ID".
func WaveKey(w domain.Wave) string {
	return domain.WaveKey(w)
}

// AvailableWaves filters waves to those available and not completed.
func AvailableWaves(waves []domain.Wave, completed map[string]bool) []domain.Wave {
	return domain.AvailableWaves(waves, completed)
}

// RestoreWaves converts WaveState slices back to Wave slices.
func RestoreWaves(states []domain.WaveState) []domain.Wave {
	return domain.RestoreWaves(states)
}

// CompletedWavesForCluster returns completed waves for a specific cluster.
func CompletedWavesForCluster(waves []domain.Wave, clusterName string) []domain.Wave {
	return domain.CompletedWavesForCluster(waves, clusterName)
}

// WaveApplyFileName returns the output filename for a wave apply result.
// Includes cluster name to avoid collisions when wave IDs are duplicated across clusters.
func WaveApplyFileName(wave domain.Wave) string {
	return fmt.Sprintf("apply_%s_%s.json", domain.SanitizeName(wave.ClusterName), domain.SanitizeName(wave.ID))
}

// RunWaveApply executes Pass 4: apply a single approved wave via Claude Code.
// It writes the apply result to a JSON file and returns the parsed result.
func RunWaveApply(ctx context.Context, cfg *domain.Config, scanDir string, wave domain.Wave, strictness string, out io.Writer, logger *domain.Logger) (*domain.WaveApplyResult, error) {
	ctx, applySpan := platform.Tracer.Start(ctx, "wave.apply",
		trace.WithAttributes(
			attribute.String("wave.id", wave.ID),
			attribute.String("wave.cluster_name", wave.ClusterName),
			attribute.Int("wave.action_count", len(wave.Actions)),
		),
	)
	defer applySpan.End()

	applyFile := filepath.Join(scanDir, WaveApplyFileName(wave))

	actionsJSON, err := json.Marshal(wave.Actions)
	if err != nil {
		return nil, fmt.Errorf("marshal wave actions: %w", err)
	}

	dodSection := domain.ResolveDoDSection(cfg.DoDTemplates, wave.ClusterName)

	prompt, err := domain.RenderWaveApplyPrompt(cfg.Lang, domain.WaveApplyPromptData{
		WaveID:          wave.ID,
		ClusterName:     wave.ClusterName,
		Title:           wave.Title,
		Actions:         string(actionsJSON),
		DoDSection:      dodSection,
		OutputPath:      applyFile,
		StrictnessLevel: strictness,
		LabelsEnabled:   cfg.Labels.Enabled,
		LabelPrefix:     cfg.Labels.Prefix,
	})
	if err != nil {
		return nil, fmt.Errorf("render apply prompt: %w", err)
	}

	// Save prompt + tee output for debugging.
	promptBase := strings.TrimSuffix(WaveApplyFileName(wave), ".json")
	if err := os.WriteFile(filepath.Join(scanDir, promptBase+"_prompt.md"), []byte(prompt), 0644); err != nil {
		logger.Warn("save apply prompt: %v", err)
	}
	applyLog, applyLogErr := os.Create(filepath.Join(scanDir, promptBase+"_output.log"))
	applyOut := out
	if applyLogErr == nil {
		defer applyLog.Close()
		applyOut = io.MultiWriter(out, applyLog)
	} else {
		logger.Warn("create apply log: %v", applyLogErr)
	}

	linearTools := WithAllowedTools(slices.Concat(BaseAllowedTools, GHAllowedTools, LinearMCPAllowedTools)...)
	logger.Info("Applying wave: %s - %s", wave.ClusterName, wave.Title)
	if _, err := RunClaudeOnce(ctx, cfg, prompt, applyOut, logger, linearTools); err != nil {
		return nil, fmt.Errorf("wave apply %s: %w", wave.ID, err)
	}

	if normErr := NormalizeJSONFile(applyFile); normErr != nil {
		logger.Warn("normalize wave apply JSON: %v", normErr)
	}
	result, err := ParseWaveApplyResult(applyFile)
	if err != nil {
		return nil, fmt.Errorf("parse apply result %s: %w", wave.ID, err)
	}

	logger.OK("Wave %s applied: %d actions", wave.ID, result.Applied)
	return result, nil
}

// RunReadyLabel applies the ready label to issues whose all waves have completed.
// This must only be called after a successful wave apply.
func RunReadyLabel(ctx context.Context, cfg *domain.Config, readyIssueIDs string, out io.Writer, logger *domain.Logger) error {
	prompt, err := domain.RenderReadyLabelPrompt(cfg.Lang, domain.ReadyLabelPromptData{
		ReadyLabel:    cfg.Labels.ReadyLabel,
		ReadyIssueIDs: readyIssueIDs,
	})
	if err != nil {
		return fmt.Errorf("render ready label prompt: %w", err)
	}

	logger.Info("Applying ready labels to: %s", readyIssueIDs)
	if _, err := RunClaudeOnce(ctx, cfg, prompt, out, logger, WithAllowedTools(LinearMCPAllowedTools...)); err != nil {
		return fmt.Errorf("ready label: %w", err)
	}
	return nil
}
