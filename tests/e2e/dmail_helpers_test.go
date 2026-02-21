//go:build e2e

package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	expect "github.com/Netflix/go-expect"
	"gopkg.in/yaml.v3"
)

// dmailData is a minimal parse struct for E2E assertions.
// E2E tests cannot import the sightjack package directly.
type dmailData struct {
	Kind        string   `yaml:"kind"`
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Severity    string   `yaml:"severity"`
	Issues      []string `yaml:"issues"`
	Body        string   `yaml:"-"` // after second "---"
}

// marshalDMail creates valid d-mail bytes (YAML frontmatter + body).
func marshalDMail(name, kind, description, severity, body string, issues []string) []byte {
	var b strings.Builder
	b.WriteString("---\n")
	fm := struct {
		Kind        string   `yaml:"kind"`
		Name        string   `yaml:"name"`
		Description string   `yaml:"description"`
		Severity    string   `yaml:"severity,omitempty"`
		Issues      []string `yaml:"issues,omitempty"`
	}{
		Kind:        kind,
		Name:        name,
		Description: description,
		Severity:    severity,
		Issues:      issues,
	}
	data, _ := yaml.Marshal(fm)
	b.Write(data)
	b.WriteString("---\n")
	if body != "" {
		b.WriteString("\n")
		b.WriteString(body)
		b.WriteString("\n")
	}
	return []byte(b.String())
}

// parseDMailFile reads a d-mail from path and parses it.
func parseDMailFile(t *testing.T, path string) *dmailData {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read d-mail %s: %v", path, err)
	}
	return parseDMailBytes(t, data)
}

// parseDMailBytes parses d-mail bytes into dmailData.
func parseDMailBytes(t *testing.T, data []byte) *dmailData {
	t.Helper()
	content := string(data)

	// Split on "---" delimiters
	if !strings.HasPrefix(content, "---\n") {
		limit := len(content)
		if limit > 40 {
			limit = 40
		}
		t.Fatalf("d-mail missing opening delimiter: %q", content[:limit])
	}
	rest := content[4:] // skip first "---\n"
	idx := strings.Index(rest, "\n---\n")
	if idx < 0 {
		if strings.HasSuffix(rest, "\n---") {
			idx = len(rest) - 4
		} else {
			t.Fatalf("d-mail missing closing delimiter")
		}
	}
	frontmatter := rest[:idx]
	body := ""
	afterClose := rest[idx+4:] // skip "\n---\n"
	if len(afterClose) > 0 {
		body = strings.TrimSpace(afterClose)
	}

	var dm dmailData
	if err := yaml.Unmarshal([]byte(frontmatter), &dm); err != nil {
		t.Fatalf("d-mail YAML parse error: %v\nfrontmatter: %s", err, frontmatter)
	}
	dm.Body = body
	return &dm
}

// listMailDir returns sorted .md filenames in .siren/{sub}/.
func listMailDir(t *testing.T, dir, sub string) []string {
	t.Helper()
	mailDir := filepath.Join(dir, ".siren", sub)
	entries, err := os.ReadDir(mailDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("read mail dir %s: %v", mailDir, err)
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	return files
}

// ensureMailDirs creates .siren/{inbox,outbox,archive}/ in dir.
func ensureMailDirs(t *testing.T, dir string) {
	t.Helper()
	for _, sub := range []string{"inbox", "outbox", "archive"} {
		p := filepath.Join(dir, ".siren", sub)
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatalf("create mail dir %s: %v", p, err)
		}
	}
}

// writeDMailToDir writes a d-mail file to the specified mail subdirectory.
func writeDMailToDir(t *testing.T, dir, sub, filename string, content []byte) string {
	t.Helper()
	p := filepath.Join(dir, ".siren", sub, filename)
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatalf("write d-mail %s: %v", p, err)
	}
	return p
}

// assertFileExists verifies a file exists at the given path.
func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("expected file to exist: %s", path)
	}
}

// assertFileNotExists verifies a file does NOT exist at the given path.
func assertFileNotExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Errorf("expected file to NOT exist: %s", path)
	}
}

// assertDirExists verifies a directory exists at the given path.
func assertDirExists(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		t.Errorf("expected directory to exist: %s", path)
		return
	}
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if !info.IsDir() {
		t.Errorf("expected %s to be a directory, got file", path)
	}
}

// runFullSession runs a full interactive session: scan → wave select → approve → apply → nextgen → quit.
func runFullSession(t *testing.T, dir string, opts ...sessionOption) {
	t.Helper()

	so := &sessionOpts{}
	for _, o := range opts {
		o(so)
	}

	c, err := expect.NewConsole(expect.WithDefaultTimeout(15 * time.Second))
	if err != nil {
		t.Fatalf("create console: %v", err)
	}
	defer c.Close()

	cmd := exec.Command(sightjackBin(), "run", dir)
	cmd.Stdin = c.Tty()
	cmd.Stdout = c.Tty()
	cmd.Stderr = c.Tty()
	if so.env != nil {
		cmd.Env = append(os.Environ(), so.env...)
	}

	if startErr := cmd.Start(); startErr != nil {
		t.Fatalf("start run: %v", startErr)
	}

	// scan → wave selection
	if _, expErr := c.ExpectString("Select wave"); expErr != nil {
		t.Fatalf("expected 'Select wave': %v", expErr)
	}

	// Hook: after first "Select wave" (fsnotify watcher is active)
	if so.afterFirstSelect != nil {
		so.afterFirstSelect()
		time.Sleep(500 * time.Millisecond)
	}

	if _, expErr := c.SendLine("1"); expErr != nil {
		t.Fatalf("send '1': %v", expErr)
	}

	// approve all
	if _, expErr := c.ExpectString("Approve all"); expErr != nil {
		t.Fatalf("expected 'Approve all': %v", expErr)
	}
	if _, expErr := c.SendLine("a"); expErr != nil {
		t.Fatalf("send 'a': %v", expErr)
	}

	// Wait for apply + report + nextgen → back to wave selection
	if _, expErr := c.ExpectString("Select wave"); expErr != nil {
		// If 2nd Select wave times out, close TTY gracefully
		c.Tty().Close()
		if _, eofErr := c.ExpectEOF(); eofErr != nil {
			t.Logf("ExpectEOF: %v", eofErr)
		}
		if waitErr := cmd.Wait(); waitErr != nil {
			if isTTYError(waitErr) {
				t.Skipf("run requires controlling terminal: %v", waitErr)
			}
		}
		t.Fatalf("expected 2nd 'Select wave' (post-nextgen): %v", expErr)
	}
	if _, expErr := c.SendLine("q"); expErr != nil {
		t.Fatalf("send 'q': %v", expErr)
	}

	c.Tty().Close()
	if _, eofErr := c.ExpectEOF(); eofErr != nil {
		t.Logf("ExpectEOF: %v", eofErr)
	}

	if waitErr := cmd.Wait(); waitErr != nil {
		if isTTYError(waitErr) {
			t.Skipf("run requires controlling terminal: %v", waitErr)
		}
		t.Fatalf("run exited with error: %v", waitErr)
	}
}

// sessionOption configures runFullSession behavior.
type sessionOption func(*sessionOpts)

type sessionOpts struct {
	env              []string
	afterFirstSelect func()
}

// withEnv adds environment variables to the session command.
func withEnv(env ...string) sessionOption {
	return func(o *sessionOpts) {
		o.env = append(o.env, env...)
	}
}

// withAfterFirstSelect sets a hook called after the first "Select wave" prompt.
// This is useful for injecting feedback files mid-session (fsnotify watcher is active).
func withAfterFirstSelect(fn func()) sessionOption {
	return func(o *sessionOpts) {
		o.afterFirstSelect = fn
	}
}
