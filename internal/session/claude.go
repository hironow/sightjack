package session

import (
	"context"
	"os/exec"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/harness"
	"github.com/hironow/sightjack/internal/platform"
)

// sharedCircuitBreaker is the process-wide circuit breaker shared across all
// provider adapter instances. Set via SetCircuitBreaker at startup.
var sharedCircuitBreaker *platform.CircuitBreaker

// SetCircuitBreaker sets the process-wide circuit breaker for all provider calls.
// Call this once during startup before any provider invocations.
func SetCircuitBreaker(cb *platform.CircuitBreaker) {
	sharedCircuitBreaker = cb
}

var newCmd = defaultNewCmd

func defaultNewCmd(ctx context.Context, name string, args ...string) *exec.Cmd {
	return platform.NewShellCmd(ctx, name, args...)
}

// OverrideNewCmd replaces the command constructor for testing and returns a
// cleanup function. Exported for cross-package test injection (root test suite).
func OverrideNewCmd(fn func(ctx context.Context, name string, args ...string) *exec.Cmd) func() {
	old := newCmd
	newCmd = fn
	return func() { newCmd = old }
}

var lookPath = platform.LookPathShell

// OverrideLookPath replaces the path lookup function for testing and returns a
// cleanup function.
func OverrideLookPath(fn func(cmd string) (string, error)) func() {
	old := lookPath
	lookPath = fn
	return func() { lookPath = old }
}

// recordCircuitBreaker updates the shared circuit breaker based on provider error classification.
func recordCircuitBreaker(provider domain.Provider, err error, stderr string) {
	if sharedCircuitBreaker == nil {
		return
	}
	if err == nil {
		sharedCircuitBreaker.RecordSuccess()
		return
	}
	// Use stderr if available, otherwise try extracting from the error message itself
	classifyTarget := stderr
	if classifyTarget == "" {
		classifyTarget = err.Error()
	}
	info := harness.ClassifyProviderError(provider, classifyTarget)
	if info.IsTrip() {
		sharedCircuitBreaker.RecordProviderError(info)
	}
}

func currentProviderState() domain.ProviderStateSnapshot {
	if sharedCircuitBreaker == nil {
		return domain.ActiveProviderState()
	}
	return sharedCircuitBreaker.Snapshot()
}
