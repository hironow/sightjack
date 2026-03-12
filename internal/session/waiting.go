package session

import (
	"context"
	"time"

	"github.com/hironow/sightjack/internal/domain"
)

// maxWaitDuration is the safety cap applied when timeout=0 (no timeout).
// Prevents indefinite process hang in unattended environments (CI/CD).
// Declared as var for test injection.
var maxWaitDuration = 24 * time.Hour

// waitForDMail blocks until a D-Mail arrives, the timeout expires, or the context is cancelled.
// Returns (true, nil) if a D-Mail arrived, (false, nil) on timeout or cancellation.
// When timeout is 0 (no timeout), maxWaitDuration is used as a safety cap.
func waitForDMail(ctx context.Context, fbCollector *FeedbackCollector, timeout time.Duration, logger domain.Logger) (arrived bool, err error) {
	logger.OK("All waves processed. Entering D-Mail waiting mode.")

	effective := timeout
	if effective <= 0 {
		effective = maxWaitDuration
		logger.Info("Waiting for incoming D-Mails... (safety cap: %s)", effective)
	} else {
		logger.Info("Waiting for incoming D-Mails... (timeout: %s)", effective)
	}
	logger.Info("Press Ctrl+C to exit.")

	t := time.NewTimer(effective)
	defer t.Stop()

	select {
	case <-ctx.Done():
		return false, nil
	case <-t.C:
		logger.Info("No D-Mails received for %s. Exiting.", effective)
		return false, nil
	case <-fbCollector.NotifyCh():
		return true, nil
	}
}
