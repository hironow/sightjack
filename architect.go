package sightjack

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

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
func RunArchitectDiscussDryRun(cfg *Config, scanDir string, wave Wave, topic string) error {
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
		StrictnessLevel: string(cfg.Strictness.Default),
	})
	if err != nil {
		return fmt.Errorf("render architect prompt: %w", err)
	}

	dryRunName := fmt.Sprintf("architect_%s_%s", sanitizeName(wave.ClusterName), sanitizeName(wave.ID))
	return RunClaudeDryRun(cfg, prompt, scanDir, dryRunName)
}

// clearArchitectOutput removes any existing architect output file to prevent
// stale results from a prior discuss round being parsed if Claude fails to
// write a new file.
func clearArchitectOutput(scanDir string, wave Wave) {
	path := filepath.Join(scanDir, architectDiscussFileName(wave))
	os.Remove(path)
}

// RunArchitectDiscuss executes a single-turn architect discussion via Claude subprocess.
func RunArchitectDiscuss(ctx context.Context, cfg *Config, scanDir string, wave Wave, topic string) (*ArchitectResponse, error) {
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
		StrictnessLevel: string(cfg.Strictness.Default),
	})
	if err != nil {
		return nil, fmt.Errorf("render architect prompt: %w", err)
	}

	LogScan("Architect discussing: %s - %s", wave.ClusterName, topic)
	if _, err := RunClaude(ctx, cfg, prompt, os.Stdout); err != nil {
		return nil, fmt.Errorf("architect discuss %s: %w", wave.ID, err)
	}

	result, err := ParseArchitectResult(outputFile)
	if err != nil {
		return nil, fmt.Errorf("parse architect result %s: %w", wave.ID, err)
	}

	return result, nil
}
