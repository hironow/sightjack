package usecase

import (
	"context"
	"fmt"
	"io"

	sightjack "github.com/hironow/sightjack"
	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/session"
)

// wrapRecorder wraps a Recorder with a DispatchingRecorder if not in dry-run mode.
func wrapRecorder(recorder domain.Recorder, logger *domain.Logger, dryRun bool) domain.Recorder {
	if dryRun {
		return recorder
	}
	engine := NewPolicyEngine(logger)
	registerSessionPolicies(engine, logger)
	return session.NewDispatchingRecorder(recorder, engine, logger)
}

// RunSession orchestrates the sightjack session pipeline.
// Validates the RunSessionCommand, then delegates to session.RunSession.
func RunSession(ctx context.Context, cmd domain.RunSessionCommand, cfg *sightjack.Config, baseDir, sessionID string, dryRun bool, input io.Reader, out io.Writer, recorder domain.Recorder, logger *domain.Logger) error {
	if errs := cmd.Validate(); len(errs) > 0 {
		return fmt.Errorf("command validation: %w", errs[0])
	}
	return session.RunSession(ctx, cfg, baseDir, sessionID, dryRun, input, out, wrapRecorder(recorder, logger, dryRun), logger)
}

// ResumeSession orchestrates the session resume pipeline.
// Validates the ResumeSessionCommand, then delegates to session.RunResumeSession.
func ResumeSession(ctx context.Context, cmd domain.ResumeSessionCommand, cfg *sightjack.Config, baseDir string, state *sightjack.SessionState, input io.Reader, out io.Writer, recorder domain.Recorder, logger *domain.Logger) error {
	if errs := cmd.Validate(); len(errs) > 0 {
		return fmt.Errorf("command validation: %w", errs[0])
	}
	return session.RunResumeSession(ctx, cfg, baseDir, state, input, out, wrapRecorder(recorder, logger, false), logger)
}

// RescanSession orchestrates the session rescan pipeline.
func RescanSession(ctx context.Context, cmd domain.RunSessionCommand, cfg *sightjack.Config, baseDir string, oldState *sightjack.SessionState, sessionID string, input io.Reader, out io.Writer, recorder domain.Recorder, logger *domain.Logger) error {
	if errs := cmd.Validate(); len(errs) > 0 {
		return fmt.Errorf("command validation: %w", errs[0])
	}
	return session.RunRescanSession(ctx, cfg, baseDir, oldState, sessionID, input, out, wrapRecorder(recorder, logger, false), logger)
}
