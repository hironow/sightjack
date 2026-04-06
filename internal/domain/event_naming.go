package domain

import "regexp"

// dotCaseRe matches the SPEC-005 event naming convention:
// lowercase ASCII, segments separated by dots, no underscores/dashes/uppercase.
// At least two segments required. Pattern: ^[a-z][a-z0-9]*(\.[a-z][a-z0-9]*)+$
var dotCaseRe = regexp.MustCompile(`^[a-z][a-z0-9]*(\.[a-z][a-z0-9]*)+$`)

// IsValidDotCaseEventType returns true if the string conforms to SPEC-005 dot.case naming.
func IsValidDotCaseEventType(s string) bool {
	return dotCaseRe.MatchString(s)
}
