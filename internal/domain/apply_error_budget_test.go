package domain_test

import (
	"testing"

	"github.com/hironow/sightjack/internal/domain"
)

func TestApplyErrorBudget_InitialState(t *testing.T) {
	// given
	budget := domain.NewApplyErrorBudget(3)

	// when/then: initial state has zero failures and is not tripped
	if budget.ConsecutiveFailures() != 0 {
		t.Errorf("ConsecutiveFailures: expected 0, got %d", budget.ConsecutiveFailures())
	}
	if budget.IsTripped() {
		t.Error("IsTripped: expected false for new budget")
	}
}

func TestApplyErrorBudget_RecordAttempt_SuccessResetsCount(t *testing.T) {
	// given
	budget := domain.NewApplyErrorBudget(3)
	budget.RecordAttempt(false) // failure
	budget.RecordAttempt(false) // failure

	// when: record a success
	budget.RecordAttempt(true)

	// then: consecutive failures reset to zero
	if budget.ConsecutiveFailures() != 0 {
		t.Errorf("after success: expected consecutive failures = 0, got %d", budget.ConsecutiveFailures())
	}
}

func TestApplyErrorBudget_RecordAttempt_FailureIncrementsCount(t *testing.T) {
	// given
	budget := domain.NewApplyErrorBudget(3)

	// when: record two failures
	budget.RecordAttempt(false)
	budget.RecordAttempt(false)

	// then
	if budget.ConsecutiveFailures() != 2 {
		t.Errorf("expected consecutive failures = 2, got %d", budget.ConsecutiveFailures())
	}
}

func TestApplyErrorBudget_CircuitBreaker_TripsAtThreshold(t *testing.T) {
	// given
	budget := domain.NewApplyErrorBudget(3)

	// when: record threshold failures
	budget.RecordAttempt(false)
	budget.RecordAttempt(false)
	budget.RecordAttempt(false)

	// then: circuit is tripped
	if !budget.IsTripped() {
		t.Error("IsTripped: expected true after reaching threshold failures")
	}
}

func TestApplyErrorBudget_CircuitBreaker_ResetsOnSuccess(t *testing.T) {
	// given: budget tripped
	budget := domain.NewApplyErrorBudget(2)
	budget.RecordAttempt(false)
	budget.RecordAttempt(false)
	if !budget.IsTripped() {
		t.Fatal("precondition: budget should be tripped")
	}

	// when: record a success
	budget.RecordAttempt(true)

	// then: circuit is reset
	if budget.IsTripped() {
		t.Error("IsTripped: expected false after success resets circuit")
	}
	if budget.ConsecutiveFailures() != 0 {
		t.Errorf("after reset: expected 0 consecutive failures, got %d", budget.ConsecutiveFailures())
	}
}
