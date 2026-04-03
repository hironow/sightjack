package session_test

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/platform"
	"github.com/hironow/sightjack/internal/session"
	"github.com/hironow/sightjack/internal/usecase/port"
)

// fakeDetailedRunner implements port.DetailedRunner for testing.
type fakeDetailedRunner struct {
	sessionID string
	text      string
	err       error
}

func (f *fakeDetailedRunner) RunDetailed(_ context.Context, _ string, _ io.Writer, _ ...port.RunOption) (port.RunResult, error) {
	return port.RunResult{Text: f.text, ProviderSessionID: f.sessionID}, f.err
}

func TestSessionTrackingAdapter_RunSession_Success(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "sessions.db")
	store, err := session.NewSQLiteCodingSessionStore(dbPath)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	defer store.Close()

	adapter := session.NewSessionTrackingAdapter(
		&fakeDetailedRunner{sessionID: "claude-sess-xyz", text: "hello world"},
		store,
		domain.ProviderClaudeCode,
	)

	rec, text, err := adapter.RunSession(ctx, "do something", io.Discard)
	if err != nil {
		t.Fatalf("RunSession: %v", err)
	}

	if text != "hello world" {
		t.Errorf("text = %q, want %q", text, "hello world")
	}
	if rec.ProviderSessionID != "claude-sess-xyz" {
		t.Errorf("ProviderSessionID = %q, want %q", rec.ProviderSessionID, "claude-sess-xyz")
	}
	if rec.Status != domain.SessionCompleted {
		t.Errorf("Status = %q, want %q", rec.Status, domain.SessionCompleted)
	}

	// Verify persisted in store
	loaded, err := store.Load(ctx, rec.ID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Status != domain.SessionCompleted {
		t.Errorf("loaded Status = %q, want %q", loaded.Status, domain.SessionCompleted)
	}
	if loaded.ProviderSessionID != "claude-sess-xyz" {
		t.Errorf("loaded ProviderSessionID = %q, want %q", loaded.ProviderSessionID, "claude-sess-xyz")
	}
}

func TestSessionTrackingAdapter_RunSession_Failure(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "sessions.db")
	store, err := session.NewSQLiteCodingSessionStore(dbPath)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	defer store.Close()

	adapter := session.NewSessionTrackingAdapter(
		&fakeDetailedRunner{sessionID: "sess-fail", text: "partial", err: context.DeadlineExceeded},
		store,
		domain.ProviderClaudeCode,
	)

	rec, text, err := adapter.RunSession(ctx, "do something", io.Discard)
	if err == nil {
		t.Fatal("RunSession should return error")
	}

	if text != "partial" {
		t.Errorf("text = %q, want %q", text, "partial")
	}
	if rec.Status != domain.SessionFailed {
		t.Errorf("Status = %q, want %q", rec.Status, domain.SessionFailed)
	}

	// Verify persisted as failed
	loaded, _ := store.Load(ctx, rec.ID)
	if loaded.Status != domain.SessionFailed {
		t.Errorf("loaded Status = %q, want %q", loaded.Status, domain.SessionFailed)
	}
	// Even on failure, provider session ID should be captured if available
	if loaded.ProviderSessionID != "sess-fail" {
		t.Errorf("loaded ProviderSessionID = %q, want %q", loaded.ProviderSessionID, "sess-fail")
	}
}

func TestSessionTrackingAdapter_RunSession_EmptyProviderSessionID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "sessions.db")
	store, err := session.NewSQLiteCodingSessionStore(dbPath)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	defer store.Close()

	adapter := session.NewSessionTrackingAdapter(
		&fakeDetailedRunner{sessionID: "", text: "ok"},
		store,
		domain.ProviderClaudeCode,
	)

	rec, _, err := adapter.RunSession(ctx, "do something", io.Discard)
	if err != nil {
		t.Fatalf("RunSession: %v", err)
	}
	if rec.ProviderSessionID != "" {
		t.Errorf("ProviderSessionID should be empty, got %q", rec.ProviderSessionID)
	}
	if rec.Status != domain.SessionCompleted {
		t.Errorf("Status = %q, want %q", rec.Status, domain.SessionCompleted)
	}
}

// --- Circuit Breaker Integration Tests ---

func TestSessionTrackingAdapter_TripsCBOnRateLimitStderr(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "sessions.db")
	store, err := session.NewSQLiteCodingSessionStore(dbPath)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	defer store.Close()

	// given — a runner that returns rate limit in stderr
	innerWithStderr := &fakeDetailedRunnerWithStderr{
		result: port.RunResult{Stderr: "You've hit your limit"},
		err:    errors.New("claude exit: exit status 1"),
	}

	cb := platform.NewCircuitBreaker(&domain.NopLogger{})
	session.SetCircuitBreaker(cb)
	defer session.SetCircuitBreaker(nil)

	adapter := session.NewSessionTrackingAdapter(innerWithStderr, store, domain.ProviderClaudeCode)

	// when
	_, _, runErr := adapter.RunSession(ctx, "test", io.Discard)

	// then — error returned AND CB tripped
	if runErr == nil {
		t.Fatal("expected error")
	}
	if !cb.IsOpen() {
		t.Fatal("expected CB OPEN after rate limit stderr")
	}
}

func TestSessionTrackingAdapter_RecordsCBSuccessOnNilError(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "sessions.db")
	store, err := session.NewSQLiteCodingSessionStore(dbPath)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	defer store.Close()

	// given — CB was tripped, then RecordSuccess to allow probe
	cb := platform.NewCircuitBreaker(&domain.NopLogger{})
	cb.RecordProviderError(domain.ProviderErrorInfo{Kind: domain.ProviderErrorRateLimit})
	cb.RecordSuccess() // transition to allow next call
	session.SetCircuitBreaker(cb)
	defer session.SetCircuitBreaker(nil)

	inner := &fakeDetailedRunnerWithStderr{
		result: port.RunResult{Text: "ok", Stderr: ""},
		err:    nil,
	}
	adapter := session.NewSessionTrackingAdapter(inner, store, domain.ProviderClaudeCode)

	// when
	_, text, runErr := adapter.RunSession(ctx, "test", io.Discard)

	// then — success recorded, CB closed
	if runErr != nil {
		t.Fatalf("expected nil, got %v", runErr)
	}
	if text != "ok" {
		t.Fatalf("expected 'ok', got %q", text)
	}
	if cb.IsOpen() {
		t.Fatal("expected CB CLOSED after success")
	}
}

func TestSessionTrackingAdapter_StoresStderrInMetadata(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "sessions.db")
	store, err := session.NewSQLiteCodingSessionStore(dbPath)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	defer store.Close()

	cb := platform.NewCircuitBreaker(&domain.NopLogger{})
	session.SetCircuitBreaker(cb)
	defer session.SetCircuitBreaker(nil)

	inner := &fakeDetailedRunnerWithStderr{
		result: port.RunResult{Stderr: "Error: 500 Internal Server Error"},
		err:    errors.New("claude exit: exit status 1"),
	}
	adapter := session.NewSessionTrackingAdapter(inner, store, domain.ProviderClaudeCode)

	// when
	rec, _, _ := adapter.RunSession(ctx, "test", io.Discard)

	// then — stderr stored in metadata
	if rec.Metadata["stderr"] != "Error: 500 Internal Server Error" {
		t.Fatalf("expected stderr in metadata, got %q", rec.Metadata["stderr"])
	}
	// CB should also be tripped (500 is a server error)
	if !cb.IsOpen() {
		t.Fatal("expected CB OPEN for 500 error")
	}
}

// fakeDetailedRunnerWithStderr extends fakeDetailedRunner with Stderr support.
type fakeDetailedRunnerWithStderr struct {
	result port.RunResult
	err    error
	calls  int
}

func (f *fakeDetailedRunnerWithStderr) Run(ctx context.Context, prompt string, w io.Writer, opts ...port.RunOption) (string, error) {
	r, err := f.RunDetailed(ctx, prompt, w, opts...)
	return r.Text, err
}

func (f *fakeDetailedRunnerWithStderr) RunDetailed(_ context.Context, _ string, _ io.Writer, _ ...port.RunOption) (port.RunResult, error) {
	f.calls++
	return f.result, f.err
}
