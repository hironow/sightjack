package sightjack

import (
	"context"
	"fmt"

	pond "github.com/alitto/pond/v2"
)

// RunParallel executes work for each item with bounded concurrency using a
// pond worker pool. Failed items produce warnings and are skipped; successful
// results are returned in the original item order.
//
// Panics inside work are recovered and converted to warnings (no deadlock).
func RunParallel[I, R any](
	ctx context.Context,
	items []I,
	concurrency int,
	work func(ctx context.Context, index int, item I) (R, error),
	itemName func(I) string,
	logger *Logger,
) ([]R, []string) {
	if len(items) == 0 {
		return nil, nil
	}
	if concurrency < 1 {
		concurrency = 1
	}

	// slot holds the outcome of a single work item.
	// Each goroutine writes to a unique index — no synchronization needed.
	type slot struct {
		result R
		err    error
		done   bool // distinguishes completed tasks from cancelled/unstarted
	}
	slots := make([]slot, len(items))

	pool := pond.NewPool(concurrency)
	group := pool.NewGroupContext(ctx)

	for i, item := range items {
		group.Submit(func() {
			// Inner panic recovery captures the error before pond's outer
			// recovery fires, preserving which task panicked and why.
			defer func() {
				if r := recover(); r != nil {
					slots[i] = slot{err: fmt.Errorf("panic: %v", r), done: true}
				}
			}()
			result, err := work(ctx, i, item)
			slots[i] = slot{result: result, err: err, done: true}
		})
	}
	_ = group.Wait()

	// Stop pool workers to prevent goroutine leaks.
	pool.StopAndWait()

	// Collect results in submission order, skipping failures and unstarted tasks.
	var results []R
	var warnings []string
	for idx, s := range slots {
		if !s.done {
			continue
		}
		if s.err != nil {
			msg := fmt.Sprintf("%q failed: %v", itemName(items[idx]), s.err)
			logger.Warn("%s", msg)
			warnings = append(warnings, msg)
		} else {
			results = append(results, s.result)
		}
	}
	return results, warnings
}
