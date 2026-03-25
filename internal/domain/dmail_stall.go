package domain

import (
	"fmt"
	"strings"
)

// StructuralErrors filters errMsg slice, returning only those classified as
// structural by ClassifyError.
func StructuralErrors(errors []string) []string {
	var result []string
	for _, e := range errors {
		if ClassifyError(e) == ErrorKindStructural {
			result = append(result, e)
		}
	}
	return result
}

// StallEscalationBody formats a d-mail body for a stall escalation message.
// It includes the wave title, the escalation reason, and the list of structural errors.
func StallEscalationBody(wave Wave, errors []string, reason string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Stall Escalation: %s\n\n", wave.Title)
	fmt.Fprintf(&b, "**Wave:** %s\n", WaveKey(wave))
	fmt.Fprintf(&b, "**Reason:** %s\n\n", reason)
	if len(errors) > 0 {
		fmt.Fprintf(&b, "## Structural Errors\n\n")
		for _, e := range errors {
			fmt.Fprintf(&b, "- %s\n", e)
		}
	}
	return b.String()
}
