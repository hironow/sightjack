package sightjack

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
)

const adrSubdir = "docs/adr"

var adrPattern = regexp.MustCompile(`^(\d{4})-.*\.md$`)

// ADRDir returns the ADR directory path under baseDir.
func ADRDir(baseDir string) string {
	return filepath.Join(baseDir, adrSubdir)
}

// NextADRNumber scans adrDir for files matching NNNN-*.md and returns max(NNNN)+1.
// Returns 1 if the directory is empty or does not exist.
func NextADRNumber(adrDir string) (int, error) {
	entries, err := os.ReadDir(adrDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 1, nil
		}
		return 0, err
	}

	maxNum := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		matches := adrPattern.FindStringSubmatch(e.Name())
		if matches == nil {
			continue
		}
		num, parseErr := strconv.Atoi(matches[1])
		if parseErr != nil {
			continue
		}
		if num > maxNum {
			maxNum = num
		}
	}

	return maxNum + 1, nil
}

// sanitizeADRTitle ensures an ADR title is safe for use in filenames.
// Prevents path traversal by stripping everything except [a-z0-9-_].
// Returns "untitled" for empty input.
func sanitizeADRTitle(title string) string {
	s := sanitizeName(title)
	if s == "" {
		return "untitled"
	}
	return s
}

// CountADRFiles returns the number of files matching NNNN-*.md in adrDir.
// Returns 0 if the directory is empty or does not exist.
func CountADRFiles(adrDir string) int {
	entries, err := os.ReadDir(adrDir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() && adrPattern.MatchString(e.Name()) {
			count++
		}
	}
	return count
}

// scribeFileName returns the output filename for a scribe run.
func scribeFileName(wave Wave) string {
	return fmt.Sprintf("scribe_%s_%s.json", sanitizeName(wave.ClusterName), sanitizeName(wave.ID))
}

// clearScribeOutput removes any existing scribe output file to prevent
// stale results from a prior run being parsed if Claude fails to write a new file.
func clearScribeOutput(scanDir string, wave Wave) {
	path := filepath.Join(scanDir, scribeFileName(wave))
	os.Remove(path)
}

// RunScribeADRDryRun saves the scribe prompt to a file instead of executing Claude.
func RunScribeADRDryRun(cfg *Config, scanDir string, wave Wave, architectResp *ArchitectResponse, adrDir string) error {
	adrNum, err := NextADRNumber(adrDir)
	if err != nil {
		return fmt.Errorf("next adr number: %w", err)
	}

	actionsJSON, err := json.Marshal(wave.Actions)
	if err != nil {
		return fmt.Errorf("marshal wave actions: %w", err)
	}

	adrID := fmt.Sprintf("%04d", adrNum)
	outputFile := filepath.Join(scanDir, scribeFileName(wave))
	prompt, err := RenderScribeADRPrompt(cfg.Lang, ScribeADRPromptData{
		ClusterName: wave.ClusterName,
		WaveTitle:   wave.Title,
		WaveActions: string(actionsJSON),
		Analysis:    architectResp.Analysis,
		Reasoning:   architectResp.Reasoning,
		ADRNumber:   adrID,
		OutputPath:  outputFile,
	})
	if err != nil {
		return fmt.Errorf("render scribe prompt: %w", err)
	}

	dryRunName := fmt.Sprintf("scribe_%s_%s", sanitizeName(wave.ClusterName), sanitizeName(wave.ID))
	return RunClaudeDryRun(cfg, prompt, scanDir, dryRunName)
}

// normalizeScribeResult ensures the parsed ADRID matches the filesystem-derived
// adrID. Claude may return a mismatched or empty adr_id; the generated ID is
// authoritative because it is used to name the ADR file on disk.
func normalizeScribeResult(result *ScribeResponse, adrID string) {
	if result.ADRID != adrID {
		if result.ADRID != "" {
			LogScan("Scribe ADR ID mismatch: generated %s, parsed %s; using %s", adrID, result.ADRID, adrID)
		}
		result.ADRID = adrID
	}
}

// ParseScribeResult reads and parses a scribe response JSON file.
func ParseScribeResult(path string) (*ScribeResponse, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read scribe result: %w", err)
	}
	var result ScribeResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse scribe result: %w", err)
	}
	return &result, nil
}

// RunScribeADR executes the Scribe Agent via Claude subprocess to generate an ADR.
func RunScribeADR(ctx context.Context, cfg *Config, scanDir string, wave Wave, architectResp *ArchitectResponse, adrDir string) (*ScribeResponse, error) {
	clearScribeOutput(scanDir, wave)

	adrNum, err := NextADRNumber(adrDir)
	if err != nil {
		return nil, fmt.Errorf("next adr number: %w", err)
	}

	actionsJSON, err := json.Marshal(wave.Actions)
	if err != nil {
		return nil, fmt.Errorf("marshal wave actions: %w", err)
	}

	adrID := fmt.Sprintf("%04d", adrNum)
	outputFile := filepath.Join(scanDir, scribeFileName(wave))
	prompt, err := RenderScribeADRPrompt(cfg.Lang, ScribeADRPromptData{
		ClusterName: wave.ClusterName,
		WaveTitle:   wave.Title,
		WaveActions: string(actionsJSON),
		Analysis:    architectResp.Analysis,
		Reasoning:   architectResp.Reasoning,
		ADRNumber:   adrID,
		OutputPath:  outputFile,
	})
	if err != nil {
		return nil, fmt.Errorf("render scribe prompt: %w", err)
	}

	LogScan("Scribe generating ADR %s for: %s - %s", adrID, wave.ClusterName, wave.Title)
	if _, err := RunClaude(ctx, cfg, prompt, os.Stdout); err != nil {
		return nil, fmt.Errorf("scribe adr %s: %w", wave.ID, err)
	}

	result, err := ParseScribeResult(outputFile)
	if err != nil {
		return nil, fmt.Errorf("parse scribe result %s: %w", wave.ID, err)
	}
	normalizeScribeResult(result, adrID)

	// Write ADR file to adrDir (sanitize title to prevent path traversal)
	if err := os.MkdirAll(adrDir, 0755); err != nil {
		return nil, fmt.Errorf("create adr dir: %w", err)
	}
	safeTitle := sanitizeADRTitle(result.Title)
	adrFileName := fmt.Sprintf("%s-%s.md", adrID, safeTitle)
	adrPath := filepath.Join(adrDir, adrFileName)
	if err := os.WriteFile(adrPath, []byte(result.Content), 0644); err != nil {
		return nil, fmt.Errorf("write adr file: %w", err)
	}

	return result, nil
}
