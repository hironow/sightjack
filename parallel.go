package sightjack

import (
	"context"
	"fmt"
)

// parallelEvent represents the outcome of a single work item.
// Goroutines send events through eventCh once work completes (success or failure).
type parallelEvent[R any] struct {
	index  int
	result R
	err    error
}

// RunParallel executes work for each item with semaphore-bounded concurrency.
//
// Flow (Command/Event):
//
//	Command phase (will-do):  dispatch goroutines for each item, gated by semaphore
//	Event phase   (did-do):   collect parallelEvent from eventCh, preserving order
//
// Failed items produce warnings and are skipped. Successful results are
// returned in the original item order.
func RunParallel[I, R any](
	ctx context.Context,
	items []I,
	concurrency int,
	work func(ctx context.Context, index int, item I) (R, error),
	itemName func(I) string,
	logger *Logger,
) ([]R, []string) {
	if concurrency < 1 {
		concurrency = 1
	}

	sem := make(chan struct{}, concurrency)
	eventCh := make(chan parallelEvent[R], len(items))

	// --- Command phase: dispatch work to goroutine pool ---
	launched := 0
	for i, item := range items {
		if ctx.Err() != nil {
			break
		}
		acquired := false
		select {
		case <-ctx.Done():
		case sem <- struct{}{}:
			acquired = true
		}
		if !acquired || ctx.Err() != nil {
			if acquired {
				<-sem
			}
			break
		}
		launched++
		go func() {
			defer func() { <-sem }()
			result, err := work(ctx, i, item)
			eventCh <- parallelEvent[R]{index: i, result: result, err: err}
		}()
	}

	// --- Event phase: collect outcomes preserving order ---
	events := make([]*parallelEvent[R], len(items))
	var warnings []string
	for range launched {
		ev := <-eventCh
		events[ev.index] = &ev
		if ev.err != nil {
			msg := fmt.Sprintf("%q failed: %v", itemName(items[ev.index]), ev.err)
			logger.Warn("%s", msg)
			warnings = append(warnings, msg)
		}
	}

	// --- Result: filter successes in original order ---
	var results []R
	for _, ev := range events {
		if ev != nil && ev.err == nil {
			results = append(results, ev.result)
		}
	}

	return results, warnings
}
