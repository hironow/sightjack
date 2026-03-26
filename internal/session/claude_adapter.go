package session

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/platform"
	"github.com/hironow/sightjack/internal/usecase/port"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// ClaudeAdapter implements port.ClaudeRunner by executing the Claude CLI
// as a subprocess. It does NOT retry; wrap with RetryRunner for that.
type ClaudeAdapter struct {
	ClaudeCmd  string
	Model      string
	TimeoutSec int
	Logger     domain.Logger
}

// Run executes the Claude CLI once without retry.
func (a *ClaudeAdapter) Run(ctx context.Context, prompt string, w io.Writer, opts ...port.RunOption) (string, error) {
	logger := a.Logger

	ctx, span := platform.Tracer.Start(ctx, "claude.invoke",
		trace.WithAttributes(
			append([]attribute.KeyValue{
				attribute.String("claude.model", platform.SanitizeUTF8(a.Model)),
				attribute.Int("claude.timeout_sec", a.TimeoutSec),
			}, platform.GenAISpanAttrs(a.Model)...)...,
		),
	)
	defer span.End()

	// Apply per-call timeout only when the caller has not already set a deadline.
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		timeout := time.Duration(a.TimeoutSec) * time.Second
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	rc := port.ApplyOptions(opts...)

	var args []string
	if a.Model != "" {
		args = append(args, "--model", a.Model)
	}
	if len(rc.AllowedTools) > 0 {
		args = append(args, "--allowedTools", strings.Join(rc.AllowedTools, ","))
	}
	if rc.Continue {
		args = append(args, "--continue")
	}
	args = append(args, "--verbose", "--output-format", "stream-json")
	args = append(args, "--disable-slash-commands")
	args = append(args, "--dangerously-skip-permissions", "--print", "-p", prompt)
	cmd := newCmd(ctx, a.ClaudeCmd, args...)
	cmd.Cancel = cancelFunc(cmd)
	cmd.WaitDelay = 3 * time.Second
	if rc.WorkDir != "" {
		cmd.Dir = rc.WorkDir
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("stdout pipe: %w", err)
	}
	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("claude start: %w", err)
	}

	var output strings.Builder
	var responseModel, responseID string
	streamErr := make(chan error, 1)
	done := make(chan struct{})

	go func() {
		defer close(done)
		sr := platform.NewStreamReader(stdout)
		if logger != nil {
			sr.SetLogger(logger)
		}
		emitter := platform.NewSpanEmittingStreamReader(sr, ctx, platform.Tracer)
		emitter.SetInput(prompt)
		result, messages, readErr := emitter.CollectAll()
		if readErr != nil {
			streamErr <- readErr
			return
		}

		for _, msg := range messages {
			switch msg.Type {
			case "assistant":
				text, _ := msg.ExtractText()
				if text != "" {
					_, _ = w.Write([]byte(text))
					output.WriteString(text)
				}
				tools, _ := msg.ExtractToolUse()
				for _, t := range tools {
					if logger != nil {
						logger.Info("  tool: %s", t.Name)
					}
				}
				if am, _ := msg.ParseAssistantMessage(); am != nil {
					if am.Model != "" {
						responseModel = am.Model
					}
					if am.ID != "" {
						responseID = am.ID
					}
				}
			case "result":
				output.Reset()
				output.WriteString(msg.Result)
				span.SetAttributes(platform.GenAIResultAttrs(msg, responseModel, responseID)...)
			}
		}

		if rawEvents := emitter.RawEvents(); len(rawEvents) > 0 {
			sanitized := make([]string, len(rawEvents))
			for i, e := range rawEvents {
				sanitized[i] = platform.SanitizeUTF8(e)
			}
			span.SetAttributes(attribute.StringSlice("stream.raw_events", platform.SanitizeUTF8Slice(sanitized)))
		}
		if result != nil && result.SessionID != "" {
			span.SetAttributes(platform.GenAISessionAttrs(result.SessionID)...)
		}

		if weaveAttrs := emitter.WeaveThreadAttrs(); len(weaveAttrs) > 0 {
			span.SetAttributes(weaveAttrs...)
		}
		if ioAttrs := emitter.WeaveIOAttrs(); len(ioAttrs) > 0 {
			span.SetAttributes(ioAttrs...)
		}
		if initAttrs := emitter.InitAttrs(); len(initAttrs) > 0 {
			span.SetAttributes(initAttrs...)
		}

		// Context budget measurement
		budget := platform.CalculateContextBudget(messages)
		span.SetAttributes(budget.Attrs()...)
		if warning := budget.WarningMessage(platform.DefaultContextBudgetThreshold); warning != "" {
			if logger != nil {
				logger.Warn("%s", warning)
			}
		}
	}()

	<-done

	var readError error
	select {
	case sErr := <-streamErr:
		readError = sErr
	default:
	}

	// Log captured stderr at debug level for diagnostics.
	if stderrBuf.Len() > 0 && logger != nil {
		logger.Debug("claude stderr:\n%s", stderrBuf.String())
	}

	if err := cmd.Wait(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			span.AddEvent("claude.timeout",
				trace.WithAttributes(attribute.Int("claude.timeout_sec", a.TimeoutSec)),
			)
		}
		return output.String(), fmt.Errorf("claude exit: %w", err)
	}

	if readError != nil {
		return output.String(), fmt.Errorf("stream read: %w", readError)
	}

	return output.String(), nil
}
