package sightjack

import (
	"context"
	"os/exec"

	"go.opentelemetry.io/otel/trace"
)

// SetNewCmd replaces the command constructor for testing and returns a cleanup function.
// This is test infrastructure for injecting fake commands, not a logic shim.
func SetNewCmd(fn func(ctx context.Context, name string, args ...string) *exec.Cmd) func() {
	old := newCmd
	newCmd = fn
	return func() { newCmd = old }
}

// SetTracer replaces the package-level tracer for testing and returns a cleanup function.
func SetTracer(t trace.Tracer) func() {
	old := tracer
	tracer = t
	return func() { tracer = old }
}
