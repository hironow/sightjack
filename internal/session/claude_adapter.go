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

// Run executes the Claude CLI once without retry, returning only the result text.
func (a *ClaudeAdapter) Run(ctx context.Context, prompt string, w io.Writer, opts ...port.RunOption) (string, error) {
	result, err := a.RunDetailed(ctx, prompt, w, opts...)
	return result.Text, err
}

// RunDetailed executes the Claude CLI once without retry, returning the result
// text and provider session ID.
func (a *ClaudeAdapter) RunDetailed(ctx context.Context, prompt string, w io.Writer, opts ...port.RunOption) (port.RunResult, error) {
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
	if rc.ResumeSessionID != "" {
		args = append(args, "--resume", rc.ResumeSessionID)
	} else if rc.Continue {
		args = append(args, "--continue")
	}
	args = append(args, "--verbose", "--output-format", "stream-json")
	// NOTE: --setting-sources "" skips settings loading but does NOT suppress CLAUDE.md auto-discovery.
	// --bare would suppress it but also disables OAuth. No individual flag exists to disable CLAUDE.md
	// discovery without disabling OAuth. Acceptable tradeoff: CLAUDE.md adds context but doesn't
	// cause context budget issues in practice.
	args = append(args, "--setting-sources", "") // Skip user/project settings (hooks, plugins, auto-memory) while preserving OAuth auth
	args = append(args, "--disable-slash-commands")

	// Settings and MCP config live under the tool's stateDir (e.g. .siren/).
	// ConfigBase is the repo root where stateDir was initialized.
	// When ConfigBase is unset, fall back to WorkDir, then CWD.
	configBase := rc.ConfigBase
	if configBase == "" {
		configBase = effectiveWorkDir(rc.WorkDir)
	}

	// Load tool-specific settings when available; warn if missing
	if settingsPath := ClaudeSettingsPath(configBase); ClaudeSettingsExists(configBase) {
		args = append(args, "--settings", settingsPath)
	} else if logger != nil {
		logger.Warn("Claude subprocess settings not found at %s", settingsPath)
		logger.Warn("Run 'sightjack mcp-config generate' to create settings.")
	}

	// Enforce MCP allowlist when .mcp.json (or legacy .run/mcp-config.json) exists
	if mcpPath := ResolveMCPConfigPath(configBase); mcpPath != "" {
		args = append(args, "--strict-mcp-config", "--mcp-config", mcpPath)
	}
	args = append(args, "--dangerously-skip-permissions", "--print", "-p", prompt)
	cmd := newCmd(ctx, a.ClaudeCmd, args...)
	cmd.Cancel = cancelFunc(cmd)
	cmd.WaitDelay = 3 * time.Second
	if rc.WorkDir != "" {
		cmd.Dir = rc.WorkDir
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return port.RunResult{}, fmt.Errorf("stdout pipe: %w", err)
	}
	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return port.RunResult{}, fmt.Errorf("claude start: %w", err)
	}

	var output strings.Builder
	var responseModel, responseID string
	var providerSessionID string
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
			providerSessionID = result.SessionID
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

		// Phase 5: persist raw events to .run/claude-logs/
		if raw := emitter.RawEvents(); len(raw) > 0 {
			if logErr := WriteClaudeLog(effectiveWorkDir(rc.WorkDir), raw); logErr != nil && logger != nil {
				logger.Warn("claude-log write: %v", logErr)
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
		return port.RunResult{Text: output.String(), ProviderSessionID: providerSessionID}, fmt.Errorf("claude exit: %w", err)
	}

	if readError != nil {
		return port.RunResult{Text: output.String(), ProviderSessionID: providerSessionID}, fmt.Errorf("stream read: %w", readError)
	}

	return port.RunResult{Text: output.String(), ProviderSessionID: providerSessionID}, nil
}

// effectiveWorkDir returns dir if non-empty, otherwise ".".
func effectiveWorkDir(dir string) string {
	if dir != "" {
		return dir
	}
	return "."
}
