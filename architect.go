package sightjack

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// ToDiscussResult converts an ArchitectResponse to the pipe wire format DiscussResult.
// It compares original and modified wave actions to build the modifications list,
// detecting changed, added, and removed actions.
func ToDiscussResult(wave Wave, resp *ArchitectResponse, topic string) DiscussResult {
	var mods []WaveModification
	if resp.ModifiedWave != nil {
		origLen := len(wave.Actions)
		modLen := len(resp.ModifiedWave.Actions)

		// Compare actions that exist in both original and modified.
		commonLen := min(origLen, modLen)
		for i := 0; i < commonLen; i++ {
			orig := wave.Actions[i]
			mod := resp.ModifiedWave.Actions[i]
			if orig.Description != mod.Description || orig.Detail != mod.Detail || orig.Type != mod.Type || orig.IssueID != mod.IssueID {
				mods = append(mods, WaveModification{
					ActionIndex: i,
					Change:      fmt.Sprintf("%s → %s", orig.Description, mod.Description),
				})
			}
		}

		// Report added actions (modified has more than original).
		for i := origLen; i < modLen; i++ {
			mod := resp.ModifiedWave.Actions[i]
			mods = append(mods, WaveModification{
				ActionIndex: i,
				Change:      fmt.Sprintf("added: %s", mod.Description),
			})
		}

		// Report removed actions (original has more than modified).
		for i := modLen; i < origLen; i++ {
			orig := wave.Actions[i]
			mods = append(mods, WaveModification{
				ActionIndex: i,
				Change:      fmt.Sprintf("removed: %s", orig.Description),
			})
		}
	}

	decision := resp.Decision
	if decision == "" {
		decision = topic
	}

	return DiscussResult{
		WaveID:        wave.ID,
		Analysis:      resp.Analysis,
		Reasoning:     resp.Reasoning,
		Decision:      decision,
		Modifications: mods,
	}
}

// ParseArchitectResult reads and parses an architect response JSON file.
func ParseArchitectResult(path string) (*ArchitectResponse, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read architect result: %w", err)
	}
	var result ArchitectResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse architect result: %w", err)
	}
	return &result, nil
}

// architectDiscussFileName returns the output filename for an architect discussion.
func architectDiscussFileName(wave Wave) string {
	return fmt.Sprintf("architect_%s_%s.json", sanitizeName(wave.ClusterName), sanitizeName(wave.ID))
}

// RunArchitectDiscussDryRun saves the architect prompt to a file instead of executing Claude.
func RunArchitectDiscussDryRun(cfg *Config, scanDir string, wave Wave, topic string, strictness string, logger *Logger) error {
	actionsJSON, err := json.Marshal(wave.Actions)
	if err != nil {
		return fmt.Errorf("marshal wave actions: %w", err)
	}

	outputFile := filepath.Join(scanDir, architectDiscussFileName(wave))
	prompt, err := RenderArchitectDiscussPrompt(cfg.Lang, ArchitectDiscussPromptData{
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

	dryRunName := fmt.Sprintf("architect_%s_%s", sanitizeName(wave.ClusterName), sanitizeName(wave.ID))
	return RunClaudeDryRun(cfg, prompt, scanDir, dryRunName, logger)
}

// clearArchitectOutput removes any existing architect output file to prevent
// stale results from a prior discuss round being parsed if Claude fails to
// write a new file.
func clearArchitectOutput(scanDir string, wave Wave) {
	path := filepath.Join(scanDir, architectDiscussFileName(wave))
	os.Remove(path)
}

// RunArchitectDiscuss executes a single-turn architect discussion via Claude subprocess.
func RunArchitectDiscuss(ctx context.Context, cfg *Config, scanDir string, wave Wave, topic string, strictness string, out io.Writer, logger *Logger) (*ArchitectResponse, error) {
	ctx, discussSpan := tracer.Start(ctx, "architect.discuss",
		trace.WithAttributes(
			attribute.String("wave.cluster_name", wave.ClusterName),
			attribute.String("wave.id", wave.ID),
		),
	)
	defer discussSpan.End()

	clearArchitectOutput(scanDir, wave)
	outputFile := filepath.Join(scanDir, architectDiscussFileName(wave))

	actionsJSON, err := json.Marshal(wave.Actions)
	if err != nil {
		return nil, fmt.Errorf("marshal wave actions: %w", err)
	}

	prompt, err := RenderArchitectDiscussPrompt(cfg.Lang, ArchitectDiscussPromptData{
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

	logger.Scan("Architect discussing: %s - %s", wave.ClusterName, topic)
	if _, err := RunClaude(ctx, cfg, prompt, out, logger, WithAllowedTools(LinearMCPAllowedTools...)); err != nil {
		return nil, fmt.Errorf("architect discuss %s: %w", wave.ID, err)
	}

	result, err := ParseArchitectResult(outputFile)
	if err != nil {
		return nil, fmt.Errorf("parse architect result %s: %w", wave.ID, err)
	}

	return result, nil
}
