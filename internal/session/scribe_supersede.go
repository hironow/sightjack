package session

import (
	"fmt"
	"os"
	"strings"
)

// SupersedeADR patches the **Status:** line of the ADR file at adrPath,
// replacing it with "Superseded by [supersededBy]".
// Returns an error if the file cannot be read/written or if no Status line is found.
func SupersedeADR(adrPath, supersededBy string) error {
	data, err := os.ReadFile(adrPath)
	if err != nil {
		return fmt.Errorf("supersede adr: read %s: %w", adrPath, err)
	}

	content := string(data)
	lines := strings.Split(content, "\n")
	found := false
	for i, line := range lines {
		if strings.HasPrefix(line, "**Status:**") {
			lines[i] = fmt.Sprintf("**Status:** Superseded by [%s]", supersededBy)
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("supersede adr: no **Status:** line found in %s", adrPath)
	}

	updated := strings.Join(lines, "\n")
	if err := os.WriteFile(adrPath, []byte(updated), 0644); err != nil {
		return fmt.Errorf("supersede adr: write %s: %w", adrPath, err)
	}
	return nil
}
