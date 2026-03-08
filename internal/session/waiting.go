package session

import (
	"context"
	"time"

	"github.com/hironow/sightjack/internal/domain"
)

// waitForDMail blocks until a D-Mail arrives, the timeout expires, or the context is cancelled.
// Returns (true, nil) if a D-Mail arrived, (false, nil) on timeout or cancellation.
func waitForDMail(ctx context.Context, fbCollector *FeedbackCollector, timeout time.Duration, logger domain.Logger) (arrived bool, err error) {
	logger.OK("All waves processed. Entering D-Mail waiting mode.")
	if timeout > 0 {
		logger.Info("Waiting for incoming D-Mails... (timeout: %s)", timeout)
	} else {
		logger.Info("Waiting for incoming D-Mails... (no timeout)")
	}
	logger.Info("Press Ctrl+C to exit.")

	var timer <-chan time.Time
	if timeout > 0 {
		t := time.NewTimer(timeout)
		defer t.Stop()
		timer = t.C
	}

	select {
	case <-ctx.Done():
		return false, nil
	case <-timer:
		logger.Info("No D-Mails received for %s. Exiting.", timeout)
		return false, nil
	case <-fbCollector.NotifyCh():
		return true, nil
	}
}
