package usecase

import (
	"context"
	"fmt"
	"io"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/port"
	"github.com/hironow/sightjack/internal/session"
)

// wrapRecorder wraps a Recorder with a DispatchingRecorder if not in dry-run mode.
func wrapRecorder(recorder domain.Recorder, logger domain.Logger, dryRun bool, cfg *domain.Config) domain.Recorder {
	if dryRun {
		return recorder
	}
	engine := NewPolicyEngine(logger)
	notifier := session.BuildNotifier(cfg)
	registerSessionPolicies(engine, logger, notifier, port.NopPolicyMetrics{})
	return session.NewDispatchingRecorder(recorder, engine, logger)
}

// RunSession orchestrates the sightjack session pipeline.
// Validates the RunSessionCommand, then delegates to session.RunSession.
func RunSession(ctx context.Context, cmd domain.RunSessionCommand, cfg *domain.Config, baseDir, sessionID string, dryRun bool, input io.Reader, out io.Writer, recorder domain.Recorder, logger domain.Logger) error {
	if errs := cmd.Validate(); len(errs) > 0 {
		return fmt.Errorf("command validation: %w", errs[0])
	}
	return session.RunSession(ctx, cfg, baseDir, sessionID, dryRun, input, out, wrapRecorder(recorder, logger, dryRun, cfg), logger)
}

// ResumeSession orchestrates the session resume pipeline.
// Validates the ResumeSessionCommand, then delegates to session.RunResumeSession.
func ResumeSession(ctx context.Context, cmd domain.ResumeSessionCommand, cfg *domain.Config, baseDir string, state *domain.SessionState, input io.Reader, out io.Writer, recorder domain.Recorder, logger domain.Logger) error {
	if errs := cmd.Validate(); len(errs) > 0 {
		return fmt.Errorf("command validation: %w", errs[0])
	}
	return session.RunResumeSession(ctx, cfg, baseDir, state, input, out, wrapRecorder(recorder, logger, false, cfg), logger)
}

// RescanSession orchestrates the session rescan pipeline.
func RescanSession(ctx context.Context, cmd domain.RunSessionCommand, cfg *domain.Config, baseDir string, oldState *domain.SessionState, sessionID string, input io.Reader, out io.Writer, recorder domain.Recorder, logger domain.Logger) error {
	if errs := cmd.Validate(); len(errs) > 0 {
		return fmt.Errorf("command validation: %w", errs[0])
	}
	return session.RunRescanSession(ctx, cfg, baseDir, oldState, sessionID, input, out, wrapRecorder(recorder, logger, false, cfg), logger)
}
