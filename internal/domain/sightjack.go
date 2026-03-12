package domain

import (
	"errors"
	"fmt"
)

// DeviationError is returned when a scan detects issues (deviation from spec).
// Callers can use errors.As to distinguish deviation from runtime errors.
type DeviationError struct {
	TotalIssues int
}

func (e *DeviationError) Error() string {
	return fmt.Sprintf("deviation detected: %d issue(s)", e.TotalIssues)
}

// ExitCode maps an error to a process exit code.
//
//	nil             → 0 (success)
//	DeviationError  → 2 (deviation detected)
//	other           → 1 (runtime error)
// SilentError wraps an error whose message has already been printed to stderr
// by the command itself. main.go should suppress output for this error
// while still honouring the exit code via ExitCode.
type SilentError struct{ Err error }

func (e *SilentError) Error() string { return e.Err.Error() }
func (e *SilentError) Unwrap() error { return e.Err }

func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var de *DeviationError
	if errors.As(err, &de) {
		return 2
	}
	return 1
}
