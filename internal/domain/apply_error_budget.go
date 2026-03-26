package domain

// ApplyErrorBudget implements a circuit-breaker pattern for wave apply operations.
// It tracks consecutive failures and trips the circuit when the threshold is reached.
// A single success resets the circuit and the consecutive failure count.
type ApplyErrorBudget struct {
	threshold           int
	consecutiveFailures int
}

// NewApplyErrorBudget creates a new ApplyErrorBudget with the given failure threshold.
// The circuit trips when consecutiveFailures >= threshold.
func NewApplyErrorBudget(threshold int) *ApplyErrorBudget {
	return &ApplyErrorBudget{threshold: threshold}
}

// RecordAttempt records the result of a single apply attempt.
// If success is true, consecutive failures are reset to zero.
// If success is false, consecutive failures are incremented.
func (b *ApplyErrorBudget) RecordAttempt(success bool) {
	if success {
		b.consecutiveFailures = 0
	} else {
		b.consecutiveFailures++
	}
}

// ConsecutiveFailures returns the current count of consecutive failures.
func (b *ApplyErrorBudget) ConsecutiveFailures() int {
	return b.consecutiveFailures
}

// IsTripped returns true when the consecutive failure count has reached or
// exceeded the threshold, indicating the circuit breaker has opened.
func (b *ApplyErrorBudget) IsTripped() bool {
	return b.consecutiveFailures >= b.threshold
}
