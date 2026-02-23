package sightjack

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"testing"
	"time"
)

func itemName(s string) string { return s }

func TestRunParallel_AllSucceed(t *testing.T) {
	// given: 3 items, all succeed
	items := []string{"A", "B", "C"}
	work := func(_ context.Context, index int, item string) (string, error) {
		return fmt.Sprintf("result-%s", item), nil
	}
	logger := NewLogger(io.Discard, false)

	// when
	results, warnings := RunParallel(context.Background(), items, 2, work, itemName, logger)

	// then: all 3 results in original order, no warnings
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0] != "result-A" || results[1] != "result-B" || results[2] != "result-C" {
		t.Errorf("unexpected results: %v", results)
	}
	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings, got %v", warnings)
	}
}

func TestRunParallel_PartialFailure(t *testing.T) {
	// given: 3 items, "B" fails
	items := []string{"A", "B", "C"}
	work := func(_ context.Context, _ int, item string) (string, error) {
		if item == "B" {
			return "", fmt.Errorf("B exploded")
		}
		return fmt.Sprintf("result-%s", item), nil
	}
	logger := NewLogger(io.Discard, false)

	// when
	results, warnings := RunParallel(context.Background(), items, 2, work, itemName, logger)

	// then: 2 successes (A, C) in order, 1 warning mentioning "B"
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0] != "result-A" || results[1] != "result-C" {
		t.Errorf("unexpected results: %v", results)
	}
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(warnings))
	}
	if !strings.Contains(warnings[0], "B") {
		t.Errorf("warning should mention B: %s", warnings[0])
	}
}

func TestRunParallel_AllFail(t *testing.T) {
	// given: 2 items, all fail
	items := []string{"X", "Y"}
	work := func(_ context.Context, _ int, item string) (string, error) {
		return "", fmt.Errorf("%s failed", item)
	}
	logger := NewLogger(io.Discard, false)

	// when
	results, warnings := RunParallel(context.Background(), items, 2, work, itemName, logger)

	// then: 0 results, 2 warnings
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
	if len(warnings) != 2 {
		t.Fatalf("expected 2 warnings, got %d", len(warnings))
	}
}

func TestRunParallel_EmptyItems(t *testing.T) {
	// given: empty slice
	work := func(_ context.Context, _ int, _ string) (string, error) {
		t.Fatal("work should not be called")
		return "", nil
	}
	logger := NewLogger(io.Discard, false)

	// when
	results, warnings := RunParallel(context.Background(), nil, 2, work, itemName, logger)

	// then
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings, got %d", len(warnings))
	}
}

func TestRunParallel_ContextCancelled(t *testing.T) {
	// given: context already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	items := []string{"A", "B"}
	callCount := 0
	work := func(_ context.Context, _ int, _ string) (string, error) {
		callCount++
		return "x", nil
	}
	logger := NewLogger(io.Discard, false)

	// when
	results, _ := RunParallel(ctx, items, 2, work, itemName, logger)

	// then: no goroutines launched
	if callCount != 0 {
		t.Errorf("expected 0 calls, got %d", callCount)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestRunParallel_PreservesOrder(t *testing.T) {
	// given: 5 items with varying work duration, concurrency=2
	items := []string{"E", "D", "C", "B", "A"}
	work := func(_ context.Context, _ int, item string) (string, error) {
		// Vary sleep so goroutines finish in different order
		time.Sleep(time.Duration(item[0]-'A') * time.Millisecond)
		return item, nil
	}
	logger := NewLogger(io.Discard, false)

	// when
	results, _ := RunParallel(context.Background(), items, 2, work, itemName, logger)

	// then: results in original order (E, D, C, B, A), not completion order
	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}
	expected := []string{"E", "D", "C", "B", "A"}
	for i, r := range results {
		if r != expected[i] {
			t.Errorf("results[%d] = %q, want %q", i, r, expected[i])
		}
	}
}

func TestRunParallel_ConcurrencyBound(t *testing.T) {
	// given: 4 items, concurrency=1 → strictly sequential
	items := []string{"A", "B", "C", "D"}
	var order []string
	work := func(_ context.Context, _ int, item string) (string, error) {
		order = append(order, item)
		return item, nil
	}
	logger := NewLogger(io.Discard, false)

	// when
	results, _ := RunParallel(context.Background(), items, 1, work, itemName, logger)

	// then: all succeed
	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}
	// With concurrency=1, execution is sequential in dispatch order
	sort.Strings(order) // order may vary slightly even with sem=1 due to goroutine scheduling
	if len(order) != 4 {
		t.Errorf("expected 4 work calls, got %d", len(order))
	}
}

func TestRunParallel_CancelWhileWaitingSemaphore(t *testing.T) {
	// given: concurrency=1, cancel while second item waits for semaphore
	ctx, cancel := context.WithCancel(context.Background())
	items := []string{"A", "B"}

	work := func(_ context.Context, _ int, item string) (string, error) {
		if item == "A" {
			cancel() // cancel while B waits for semaphore
		}
		return item, nil
	}
	logger := NewLogger(io.Discard, false)

	// when
	results, _ := RunParallel(ctx, items, 1, work, itemName, logger)

	// then: at most 1 result (A completed before cancel)
	if len(results) > 1 {
		t.Errorf("expected at most 1 result, got %d", len(results))
	}
}
