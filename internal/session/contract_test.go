//go:build contract

package session
// white-box-reason: contract validation: tests unexported golden file enumeration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const contractGoldenDir = "testdata/contract"

func contractGoldenFiles(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(contractGoldenDir)
	if err != nil {
		t.Fatalf("read contract golden dir: %v", err)
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			files = append(files, e.Name())
		}
	}
	if len(files) == 0 {
		t.Fatal("no contract golden files found")
	}
	return files
}

func readContractGolden(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(contractGoldenDir, name))
	if err != nil {
		t.Fatalf("read contract golden %s: %v", name, err)
	}
	return data
}

// TestContract_ParseDMail verifies that sightjack's ParseDMail can
// parse all cross-tool golden files. Sightjack is Postel-liberal at
// the parse level — unknown kinds and future schemas parse without error.
func TestContract_ParseDMail(t *testing.T) {
	for _, name := range contractGoldenFiles(t) {
		t.Run(name, func(t *testing.T) {
			data := readContractGolden(t, name)
			dm, err := ParseDMail(data)
			if err != nil {
				t.Fatalf("ParseDMail error: %v", err)
			}
			if dm.Name == "" {
				t.Error("parsed name is empty")
			}
			if dm.Kind == "" {
				t.Error("parsed kind is empty")
			}
			if dm.Description == "" {
				t.Error("parsed description is empty")
			}
			if dm.SchemaVersion == "" {
				t.Error("parsed schema version is empty")
			}
		})
	}
}

// TestContract_ValidateDMailRejectsEdgeCases verifies that sightjack's
// strict validation rejects D-Mails with unknown kinds.
func TestContract_ValidateDMailRejectsEdgeCases(t *testing.T) {
	data := readContractGolden(t, "unknown-kind.md")
	dm, err := ParseDMail(data)
	if err != nil {
		t.Fatalf("ParseDMail error: %v", err)
	}
	// Parse succeeds, but validation should reject unknown kind
	if err := ValidateDMail(dm); err == nil {
		t.Error("expected ValidateDMail to fail for unknown kind 'advisory', but it passed")
	}
}
