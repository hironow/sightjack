package domain_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
)

func TestExitCode_Nil(t *testing.T) {
	if code := domain.ExitCode(nil); code != 0 {
		t.Errorf("ExitCode(nil) = %d, want 0", code)
	}
}

func TestExitCode_DeviationError(t *testing.T) {
	err := &domain.DeviationError{TotalIssues: 5}
	if code := domain.ExitCode(err); code != 2 {
		t.Errorf("ExitCode(DeviationError) = %d, want 2", code)
	}
}

func TestExitCode_RegularError(t *testing.T) {
	err := fmt.Errorf("something broke")
	if code := domain.ExitCode(err); code != 1 {
		t.Errorf("ExitCode(regular) = %d, want 1", code)
	}
}

func TestExitCode_WrappedDeviationError(t *testing.T) {
	inner := &domain.DeviationError{TotalIssues: 3}
	wrapped := fmt.Errorf("scan: %w", inner)

	if code := domain.ExitCode(wrapped); code != 2 {
		t.Errorf("ExitCode(wrapped DeviationError) = %d, want 2", code)
	}
}

func TestDeviationError_IsError(t *testing.T) {
	err := &domain.DeviationError{TotalIssues: 2}
	if err.Error() == "" {
		t.Error("DeviationError.Error() should not be empty")
	}
}

func TestDeviationError_Unwrap(t *testing.T) {
	var de *domain.DeviationError
	err := fmt.Errorf("wrap: %w", &domain.DeviationError{TotalIssues: 1})
	if !errors.As(err, &de) {
		t.Error("errors.As should find DeviationError in chain")
	}
}

func TestSilentError_ExitCode(t *testing.T) {
	// given — SilentError wrapping a regular error
	err := &domain.SilentError{Err: fmt.Errorf("1 check(s) failed")}

	// then — ExitCode should return 1 (runtime error)
	if code := domain.ExitCode(err); code != 1 {
		t.Errorf("ExitCode(SilentError) = %d, want 1", code)
	}
}

func TestSilentError_Unwrap(t *testing.T) {
	// given
	inner := fmt.Errorf("inner cause")
	err := &domain.SilentError{Err: inner}

	// then
	if !errors.Is(err, inner) {
		t.Error("errors.Is should find inner error through SilentError")
	}
}

func TestSilentError_DetectedByErrorsAs(t *testing.T) {
	// given
	err := fmt.Errorf("command: %w", &domain.SilentError{Err: fmt.Errorf("fail")})

	// then
	var se *domain.SilentError
	if !errors.As(err, &se) {
		t.Error("errors.As should find SilentError in chain")
	}
}
