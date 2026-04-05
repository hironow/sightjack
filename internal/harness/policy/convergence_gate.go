package policy

import "strings"

// MaxConvergenceRedrainCycles caps how many approval/redrain cycles a
// convergence gate may perform before failing closed.
const MaxConvergenceRedrainCycles = 3

// IsConvergenceKind reports whether a D-Mail kind should trigger the
// convergence gate.
func IsConvergenceKind(kind string) bool {
	return kind == "convergence"
}

// BuildConvergenceSummary formats the approval summary for convergence D-Mails.
func BuildConvergenceSummary(names []string) string {
	return "[CONVERGENCE] Convergence signal received: " + strings.Join(names, ", ")
}
