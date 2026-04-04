package verifier

import (
	"strings"
)

// IsRateLimited checks if the review output indicates a rate/quota limit.
func IsRateLimited(output string) bool {
	lower := strings.ToLower(output)
	signals := []string{
		"rate limit",
		"rate_limit",
		"quota exceeded",
		"quota limit",
		"too many requests",
		"usage limit",
	}
	for _, s := range signals {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}
