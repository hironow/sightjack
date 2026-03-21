package session

import (
	"fmt"
	"strings"

	"github.com/hironow/sightjack/internal/domain"
)

// FormatADRConflictSection formats the conflicts from a ScribeResponse into
// a Markdown section suitable for appending to the ADR content.
// Returns an empty string if there are no conflicts.
func FormatADRConflictSection(resp *domain.ScribeResponse) string {
	if resp == nil || len(resp.Conflicts) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "\n## Conflicts\n\n")
	for _, c := range resp.Conflicts {
		fmt.Fprintf(&b, "- **ADR %s**: %s\n", c.ExistingADRID, c.Description)
	}
	return b.String()
}
