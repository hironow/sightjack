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

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// ToDiscussResult converts an sightjack.ArchitectResponse to the pipe wire format sightjack.DiscussResult.
// It compares original and modified wave actions to build the modifications list,
// detecting changed, added, and removed actions.
func ToDiscussResult(wave sightjack.Wave, resp *sightjack.ArchitectResponse, topic string) sightjack.DiscussResult {
	var mods []sightjack.WaveModification
	if resp.ModifiedWave != nil {
		origLen := len(wave.Actions)
		modLen := len(resp.ModifiedWave.Actions)

		// Compare actions that exist in both original and modified.
		commonLen := min(origLen, modLen)
		for i := 0; i < commonLen; i++ {
			orig := wave.Actions[i]
			mod := resp.ModifiedWave.Actions[i]
			if orig.Description != mod.Description || orig.Detail != mod.Detail || orig.Type != mod.Type || orig.IssueID != mod.IssueID {
				mods = append(mods, sightjack.WaveModification{
					ActionIndex: i,
					Change:      fmt.Sprintf("%s → %s", orig.Description, mod.Description),
				})
			}
		}

		// Report added actions (modified has more than original).
		for i := origLen; i < modLen; i++ {
			mod := resp.ModifiedWave.Actions[i]
			mods = append(mods, sightjack.WaveModification{
				ActionIndex: i,
				Change:      fmt.Sprintf("added: %s", mod.Description),
			})
		}

		// Report removed actions (original has more than modified).
		for i := modLen; i < origLen; i++ {
			orig := wave.Actions[i]
			mods = append(mods, sightjack.WaveModification{
				ActionIndex: i,
				Change:      fmt.Sprintf("removed: %s", orig.Description),
			})
		}
	}

	decision := resp.Decision
	if decision == "" {
		decision = topic
	}

	return sightjack.DiscussResult{
		WaveID:        wave.ID,
		Analysis:      resp.Analysis,
		Reasoning:     resp.Reasoning,
		Decision:      decision,
		Modifications: mods,
	}
}

// ParseArchitectResult reads and parses an architect response JSON file.
func ParseArchitectResult(path string) (*sightjack.ArchitectResponse, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read architect result: %w", err)
	}
	var result sightjack.ArchitectResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse architect result: %w", err)
	}
	return &result, nil
}

// ArchitectDiscussFileName returns the output filename for an architect discussion.
func ArchitectDiscussFileName(wave sightjack.Wave) string {
	return fmt.Sprintf("architect_%s_%s.json", domain.SanitizeName(wave.ClusterName), domain.SanitizeName(wave.ID))
}

// RunArchitectDiscussDryRun saves the architect prompt to a file instead of executing Claude.
func RunArchitectDiscussDryRun(cfg *sightjack.Config, scanDir string, wave sightjack.Wave, topic string, strictness string, logger *sightjack.Logger) error {
	actionsJSON, err := json.Marshal(wave.Actions)
	if err != nil {
		return fmt.Errorf("marshal wave actions: %w", err)
	}

	outputFile := filepath.Join(scanDir, ArchitectDiscussFileName(wave))
	prompt, err := sightjack.RenderArchitectDiscussPrompt(cfg.Lang, sightjack.ArchitectDiscussPromptData{
		ClusterName:     wave.ClusterName,
		WaveTitle:       wave.Title,
		WaveActions:     string(actionsJSON),
		Topic:           topic,
		OutputPath:      outputFile,
		StrictnessLevel: strictness,
	})
	if err != nil {
		return fmt.Errorf("render architect prompt: %w", err)
	}

	dryRunName := fmt.Sprintf("architect_%s_%s", domain.SanitizeName(wave.ClusterName), domain.SanitizeName(wave.ID))
	return RunClaudeDryRun(cfg, prompt, scanDir, dryRunName, logger)
}

// ClearArchitectOutput removes any existing architect output file to prevent
// stale results from a prior discuss round being parsed if Claude fails to
// write a new file.
func ClearArchitectOutput(scanDir string, wave sightjack.Wave) {
	path := filepath.Join(scanDir, ArchitectDiscussFileName(wave))
	_ = os.Remove(path)
}

// RunArchitectDiscuss executes a single-turn architect discussion via Claude subprocess.
func RunArchitectDiscuss(ctx context.Context, cfg *sightjack.Config, scanDir string, wave sightjack.Wave, topic string, strictness string, out io.Writer, logger *sightjack.Logger) (*sightjack.ArchitectResponse, error) {
	ctx, discussSpan := tracer.Start(ctx, "architect.discuss",
		trace.WithAttributes(
			attribute.String("wave.cluster_name", wave.ClusterName),
			attribute.String("wave.id", wave.ID),
		),
	)
	defer discussSpan.End()

	ClearArchitectOutput(scanDir, wave)
	outputFile := filepath.Join(scanDir, ArchitectDiscussFileName(wave))

	actionsJSON, err := json.Marshal(wave.Actions)
	if err != nil {
		return nil, fmt.Errorf("marshal wave actions: %w", err)
	}

	prompt, err := sightjack.RenderArchitectDiscussPrompt(cfg.Lang, sightjack.ArchitectDiscussPromptData{
		ClusterName:     wave.ClusterName,
		WaveTitle:       wave.Title,
		WaveActions:     string(actionsJSON),
		Topic:           topic,
		OutputPath:      outputFile,
		StrictnessLevel: strictness,
	})
	if err != nil {
		return nil, fmt.Errorf("render architect prompt: %w", err)
	}

	// Save prompt + tee output for debugging.
	promptBase := ArchitectDiscussFileName(wave)
	promptBase = strings.TrimSuffix(promptBase, ".json")
	if err := os.WriteFile(filepath.Join(scanDir, promptBase+"_prompt.md"), []byte(prompt), 0644); err != nil {
		logger.Warn("save architect prompt: %v", err)
	}
	discussLog, discussLogErr := os.Create(filepath.Join(scanDir, promptBase+"_output.log"))
	discussOut := out
	if discussLogErr == nil {
		defer discussLog.Close()
		discussOut = io.MultiWriter(out, discussLog)
	} else {
		logger.Warn("create architect log: %v", discussLogErr)
	}

	logger.Scan("Architect discussing: %s - %s", wave.ClusterName, topic)
	if _, err := RunClaude(ctx, cfg, prompt, discussOut, logger, WithAllowedTools(slices.Concat(BaseAllowedTools, GHAllowedTools, LinearMCPAllowedTools)...)); err != nil {
		return nil, fmt.Errorf("architect discuss %s: %w", wave.ID, err)
	}

	if normErr := NormalizeJSONFile(outputFile); normErr != nil {
		logger.Warn("normalize architect JSON: %v", normErr)
	}
	result, err := ParseArchitectResult(outputFile)
	if err != nil {
		return nil, fmt.Errorf("parse architect result %s: %w", wave.ID, err)
	}

	return result, nil
}
