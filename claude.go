package sightjack

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var newCmd = defaultNewCmd

func defaultNewCmd(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}

// BuildClaudeArgs constructs the argument list for the Claude CLI based on
// the given configuration and prompt text.
func BuildClaudeArgs(cfg *Config, prompt string) []string {
	args := []string{"--print"}
	if cfg.Claude.Model != "" {
		args = append(args, "--model", cfg.Claude.Model)
	}
	args = append(args, "-p", prompt)
	return args
}

// runClaudeOnce executes the Claude CLI as a subprocess once, streaming its
// output to w in real time and returning the full output when complete.
func runClaudeOnce(ctx context.Context, cfg *Config, prompt string, w io.Writer) (string, error) {
	timeout := time.Duration(cfg.Claude.TimeoutSec) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args := BuildClaudeArgs(cfg, prompt)
	cmd := newCmd(ctx, cfg.Claude.Command, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("stdout pipe: %w", err)
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("claude start: %w", err)
	}

	var output strings.Builder
	done := make(chan struct{})

	go func() {
		defer close(done)
		reader := bufio.NewReader(stdout)
		buf := make([]byte, 4096)
		for {
			n, readErr := reader.Read(buf)
			if n > 0 {
				chunk := buf[:n]
				w.Write(chunk)
				output.Write(chunk)
			}
			if readErr != nil {
				if readErr != io.EOF {
					LogWarn("stdout read: %v", readErr)
				}
				break
			}
		}
	}()

	<-done

	if err := cmd.Wait(); err != nil {
		return output.String(), fmt.Errorf("claude exit: %w", err)
	}

	return output.String(), nil
}

// RunClaude executes the Claude CLI as a subprocess with exponential backoff
// retry. It streams output to w in real time and returns the full output when
// complete.
// Pass os.Stdout for interactive single-process usage, or io.Discard for
// parallel invocations where interleaved output would be unreadable.
func RunClaude(ctx context.Context, cfg *Config, prompt string, w io.Writer) (string, error) {
	maxAttempts := cfg.Retry.MaxAttempts
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	baseDelay := time.Duration(cfg.Retry.BaseDelaySec) * time.Second

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if attempt > 1 {
			delay := baseDelay * time.Duration(1<<(attempt-2)) // exponential: base, 2*base, 4*base...
			LogInfo("Retrying (%d/%d) after %v...", attempt, maxAttempts, delay)
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(delay):
			}
		}
		output, err := runClaudeOnce(ctx, cfg, prompt, w)
		if err == nil {
			return output, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return output, err
		}
	}
	return "", fmt.Errorf("claude failed after %d attempts: %w", maxAttempts, lastErr)
}

// RunClaudeDryRun saves the prompt to a file instead of executing Claude,
// useful for previewing what would be sent. The name parameter makes each
// prompt file unique within the output directory (e.g. "classify", "wave_00_auth").
func RunClaudeDryRun(cfg *Config, prompt, outputPath string, name string) error {
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		return fmt.Errorf("create dry-run dir: %w", err)
	}
	promptFile := filepath.Join(outputPath, name+"_prompt.md")
	if err := os.WriteFile(promptFile, []byte(prompt), 0644); err != nil {
		return fmt.Errorf("write prompt: %w", err)
	}
	LogInfo("Dry-run: prompt saved to %s", promptFile)
	return nil
}
