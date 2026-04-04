package session

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/harness"
	"github.com/hironow/sightjack/internal/platform"
	"github.com/hironow/sightjack/internal/usecase/port"
)

// autoDiscussRoundResponse is the JSON output from a single auto-discuss round.
type autoDiscussRoundResponse struct {
	Content                 string   `json:"content"`
	OpenIssues              []string `json:"open_issues,omitempty"`
	ADRRecommended          *bool    `json:"adr_recommended,omitempty"`
	ADRRecommendationReason string   `json:"adr_recommendation_reason,omitempty"`
}

// autoDiscussOutputFileName returns the output filename for a single round.
func autoDiscussOutputFileName(speaker string, wave domain.Wave, round int) string {
	return fmt.Sprintf("auto_discuss_%s_%s_%s_r%d.json",
		speaker,
		domain.SanitizeName(wave.ClusterName),
		domain.SanitizeName(wave.ID),
		round,
	)
}

// RunAutoDiscuss orchestrates the Devil's Advocate debate for auto-approve mode.
// Returns nil result (not error) when auto_discuss_rounds is 0.
func RunAutoDiscuss(ctx context.Context, cfg *domain.Config, scanDir string,
	wave domain.Wave, feedback []*DMail, adrDir string, strictness string,
	out io.Writer, runner port.ClaudeRunner, logger domain.Logger) (*domain.AutoDiscussResult, error) {

	rounds := cfg.Scribe.AutoDiscussRounds
	if rounds <= 0 {
		return nil, nil
	}

	ctx, span := platform.Tracer.Start(ctx, "scribe.auto_discuss",
		trace.WithAttributes(
			attribute.String("wave.id", platform.SanitizeUTF8(wave.ID)),
			attribute.String("wave.cluster_name", platform.SanitizeUTF8(wave.ClusterName)),
			attribute.Int("discuss.rounds", rounds),
		),
	)
	defer span.End()

	existingADRs, err := ReadExistingADRs(adrDir)
	if err != nil {
		span.RecordError(err)
		logger.Warn("Auto-discuss: read ADRs failed: %v", err)
		existingADRs = nil
	}

	claudeMD := ReadCLAUDEMD(scanDir)
	if claudeMD == "" {
		logger.Info("Auto-discuss: CLAUDE.md not found, proceeding with ADRs only")
	}

	actionsJSON, err := json.Marshal(wave.Actions)
	if err != nil {
		return nil, fmt.Errorf("auto-discuss: marshal wave actions: %w", err)
	}

	feedbackSection := FormatFeedbackForPrompt(feedback)

	var allRounds []domain.AutoDiscussRound
	var priorContent string

	// Round 0: Architect explains (always)
	// Round 1..2N-1: alternating Devil's Advocate (odd) and Architect (even)
	// Total calls = 2N (N architect + N devil's advocate)
	// Final round (r=2N-1) is always devil's advocate and includes open_issues summary
	totalCalls := rounds * 2
	for r := 0; r < totalCalls; r++ {
		speaker := speakerForRound(r)
		roundCtx, roundSpan := platform.Tracer.Start(ctx, "scribe.auto_discuss.round", // nosemgrep: adr0003-otel-span-without-defer-end — End() called after round logic below (not defer: loop iteration) [permanent]
			trace.WithAttributes(
				attribute.Int("round.index", r),
				attribute.String("round.speaker", platform.SanitizeUTF8(speaker)),
			),
		)

		var content string
		var roundErr error

		if speaker == "architect" {
			content, roundErr = runAutoDiscussArchitect(roundCtx, cfg, scanDir, wave,
				string(actionsJSON), priorContent, feedbackSection, strictness, r, out, runner, logger)
		} else {
			isFinal := r == totalCalls-1
			daRoundIndex := (r + 1) / 2
			content, roundErr = runAutoDiscussDevilsAdvocate(roundCtx, cfg, scanDir, wave,
				string(actionsJSON), priorContent, existingADRs, claudeMD,
				strictness, daRoundIndex, rounds, isFinal, r, out, runner, logger)
		}

		roundSpan.End()

		if roundErr != nil {
			logger.Warn("Auto-discuss round %d (%s) failed: %v", r, speaker, roundErr)
			span.RecordError(roundErr)
			break
		}

		if content == "" {
			logger.Warn("Auto-discuss round %d (%s): empty content, stopping debate", r, speaker)
			span.AddEvent("auto_discuss.empty_content",
				trace.WithAttributes(
					attribute.Int("round.index", r),
					attribute.String("round.speaker", platform.SanitizeUTF8(speaker)),
				),
			)
			break
		}

		allRounds = append(allRounds, domain.AutoDiscussRound{
			Round:   r,
			Speaker: speaker,
			Content: content,
		})
		priorContent = content
	}

	result := &domain.AutoDiscussResult{
		Rounds: allRounds,
	}

	// Parse final devil's advocate round for open issues
	if len(allRounds) > 0 {
		last := allRounds[len(allRounds)-1]
		if last.Speaker == "devils_advocate" {
			result.OpenIssues, result.Summary = parseFinalRound(scanDir, wave, last.Round)
		}
	}
	if result.Summary == "" {
		result.Summary = buildSummaryFromRounds(allRounds)
	}

	span.SetAttributes(
		attribute.Int("discuss.open_issues", len(result.OpenIssues)),
		attribute.Int("discuss.actual_rounds", len(result.Rounds)),
	)

	return result, nil
}

func speakerForRound(r int) string {
	if r%2 == 0 {
		return "architect"
	}
	return "devils_advocate"
}

func runAutoDiscussArchitect(ctx context.Context, cfg *domain.Config, scanDir string,
	wave domain.Wave, actionsJSON, priorContent, feedbackSection, strictness string,
	roundIndex int, out io.Writer, runner port.ClaudeRunner, logger domain.Logger) (string, error) {

	outputFile := filepath.Join(scanDir, autoDiscussOutputFileName("architect", wave, roundIndex))
	_ = os.Remove(outputFile)

	prompt, err := harness.RenderAutoDiscussArchitectPrompt(cfg.Lang, domain.AutoDiscussArchitectPromptData{
		ClusterName:     wave.ClusterName,
		WaveTitle:       wave.Title,
		WaveActions:     actionsJSON,
		PriorContent:    priorContent,
		FeedbackSection: feedbackSection,
		OutputPath:      outputFile,
		StrictnessLevel: strictness,
	})
	if err != nil {
		return "", fmt.Errorf("render architect prompt: %w", err)
	}

	logger.Info("Auto-discuss: Architect (round %d)", roundIndex)
	if _, err := runner.Run(ctx, prompt, out); err != nil {
		return "", fmt.Errorf("auto-discuss architect: %w", err)
	}

	if normErr := NormalizeJSONFile(outputFile); normErr != nil {
		logger.Warn("normalize auto-discuss architect JSON: %v", normErr)
	}
	return readRoundContent(outputFile)
}

func runAutoDiscussDevilsAdvocate(ctx context.Context, cfg *domain.Config, scanDir string,
	wave domain.Wave, actionsJSON, priorContent string, existingADRs []domain.ExistingADR,
	claudeMD, strictness string, daRoundIndex, totalRounds int, isFinal bool,
	roundIndex int, out io.Writer, runner port.ClaudeRunner, logger domain.Logger) (string, error) {

	outputFile := filepath.Join(scanDir, autoDiscussOutputFileName("devils_advocate", wave, roundIndex))
	_ = os.Remove(outputFile)

	prompt, err := harness.RenderAutoDiscussDevilsAdvocatePrompt(cfg.Lang, domain.AutoDiscussDevilsAdvocatePromptData{
		ClusterName:     wave.ClusterName,
		WaveTitle:       wave.Title,
		WaveActions:     actionsJSON,
		PriorContent:    priorContent,
		ExistingADRs:    existingADRs,
		CLAUDEMDContent: claudeMD,
		OutputPath:      outputFile,
		StrictnessLevel: strictness,
		RoundIndex:      daRoundIndex,
		TotalRounds:     totalRounds,
		IsFinalRound:    isFinal,
	})
	if err != nil {
		return "", fmt.Errorf("render devils_advocate prompt: %w", err)
	}

	finalTag := ""
	if isFinal {
		finalTag = " FINAL"
	}
	logger.Info("Auto-discuss: Devil's Advocate (round %d/%d%s)", daRoundIndex, totalRounds, finalTag)
	if _, err := runner.Run(ctx, prompt, out); err != nil {
		return "", fmt.Errorf("auto-discuss devils_advocate: %w", err)
	}

	if normErr := NormalizeJSONFile(outputFile); normErr != nil {
		logger.Warn("normalize auto-discuss devils_advocate JSON: %v", normErr)
	}
	return readRoundContent(outputFile)
}

func readRoundContent(outputFile string) (string, error) {
	data, err := os.ReadFile(outputFile)
	if err != nil {
		return "", fmt.Errorf("read round output: %w", err)
	}
	var resp autoDiscussRoundResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("parse round output: %w", err)
	}
	return resp.Content, nil
}

func parseFinalRound(scanDir string, wave domain.Wave, roundIndex int) ([]string, string) {
	outputFile := filepath.Join(scanDir, autoDiscussOutputFileName("devils_advocate", wave, roundIndex))
	data, err := os.ReadFile(outputFile)
	if err != nil {
		return nil, ""
	}
	var resp autoDiscussRoundResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, ""
	}
	summary := resp.ADRRecommendationReason
	if summary == "" {
		summary = resp.Content
	}
	return resp.OpenIssues, summary
}

func buildSummaryFromRounds(rounds []domain.AutoDiscussRound) string {
	var parts []string
	for _, r := range rounds {
		parts = append(parts, fmt.Sprintf("[%s]: %s", r.Speaker, r.Content))
	}
	return strings.Join(parts, "\n\n")
}
