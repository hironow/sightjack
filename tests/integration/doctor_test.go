package integration_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/platform"
	"github.com/hironow/sightjack/internal/session"
)

// buildFakeClaude compiles the fake-claude binary and returns its absolute path.
// The binary supports --version and `mcp list` subcommands used by doctor checks.
func buildFakeClaude(t *testing.T) string {
	t.Helper()
	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "fake-claude")

	// fake-claude source is at tests/scenario/testdata/fake-claude/
	// relative to tests/integration/ (where this test runs).
	fakeSrc, err := filepath.Abs("../scenario/testdata/fake-claude")
	if err != nil {
		t.Fatalf("resolve fake-claude path: %v", err)
	}

	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir = fakeSrc
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build fake-claude: %v\n%s", err, out)
	}
	return binPath
}

func TestCheckConfig_ValidConfig(t *testing.T) {
	// given: valid config file
	dir := t.TempDir()
	path := filepath.Join(dir, "sightjack.yaml")
	os.WriteFile(path, []byte(`
tracker:
  team: "Test"
  project: "Project"
`), 0644)

	// when
	result := session.CheckConfig(path)

	// then
	if result.Status != domain.CheckOK {
		t.Errorf("expected CheckOK, got %v: %s", result.Status, result.Message)
	}
	if result.Name != "Config" {
		t.Errorf("expected name 'Config', got %q", result.Name)
	}
}

func TestCheckConfig_MissingFile(t *testing.T) {
	// given: nonexistent path
	result := session.CheckConfig("/nonexistent/sightjack.yaml")

	// then
	if result.Status != domain.CheckFail {
		t.Errorf("expected CheckFail, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckConfig_InvalidYAML(t *testing.T) {
	// given: invalid YAML content
	dir := t.TempDir()
	path := filepath.Join(dir, "sightjack.yaml")
	os.WriteFile(path, []byte(`{{{invalid yaml`), 0644)

	// when
	result := session.CheckConfig(path)

	// then
	if result.Status != domain.CheckFail {
		t.Errorf("expected CheckFail for invalid YAML, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckTool_Exists(t *testing.T) {
	// given: "git" is guaranteed to exist in dev environment and supports --version
	ctx := context.Background()

	// when
	result := session.CheckTool(ctx, "git")

	// then
	if result.Status != domain.CheckOK {
		t.Errorf("expected CheckOK for 'git', got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "git") {
		t.Errorf("expected message to contain path, got: %s", result.Message)
	}
}

func TestCheckTool_NotFound(t *testing.T) {
	// given: nonexistent tool
	ctx := context.Background()

	// when
	result := session.CheckTool(ctx, "nonexistent-tool-xyz-12345")

	// then
	if result.Status != domain.CheckFail {
		t.Errorf("expected CheckFail, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckStatusLabel(t *testing.T) {
	tests := []struct {
		status domain.CheckStatus
		want   string
	}{
		{domain.CheckOK, "OK"},
		{domain.CheckFail, "FAIL"},
		{domain.CheckSkip, "SKIP"},
	}
	for _, tt := range tests {
		if got := tt.status.StatusLabel(); got != tt.want {
			t.Errorf("StatusLabel(%d): expected %q, got %q", tt.status, tt.want, got)
		}
	}
}

func TestCheckStateDir_Writable(t *testing.T) {
	// given: a directory where .siren/ can be created
	dir := t.TempDir()

	// when
	result := session.CheckStateDir(dir, false)

	// then: without repair, missing dir = FAIL
	if result.Status != domain.CheckFail {
		t.Errorf("expected CheckFail for missing .siren/ without repair, got %v: %s", result.Status, result.Message)
	}
	if result.Name != "State Dir" {
		t.Errorf("expected name 'State Dir', got %q", result.Name)
	}
}

func TestCheckStateDir_RepairCreatesMissing(t *testing.T) {
	// given: a directory where .siren/ does not exist
	dir := t.TempDir()

	// when: repair=true
	result := session.CheckStateDir(dir, true)

	// then: should create and return Fixed
	if result.Status != domain.CheckFixed {
		t.Errorf("expected CheckFixed, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckStateDir_NotWritable(t *testing.T) {
	// given: a read-only directory
	dir := t.TempDir()
	os.Chmod(dir, 0555)
	defer os.Chmod(dir, 0755) // cleanup

	// when
	result := session.CheckStateDir(dir, true)

	// then
	if result.Status != domain.CheckFail {
		t.Errorf("expected CheckFail for read-only dir, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckStateDir_ExistingDir(t *testing.T) {
	// given: .siren/ already exists and is writable
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".siren"), 0755)

	// when
	result := session.CheckStateDir(dir, false)

	// then
	if result.Status != domain.CheckOK {
		t.Errorf("expected CheckOK for existing .siren/, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckSkills_OK(t *testing.T) {
	// given: valid SKILL.md files installed
	baseDir := t.TempDir()
	if err := session.InstallSkills(baseDir, platform.SkillsFS, &domain.NopLogger{}); err != nil {
		t.Fatalf("InstallSkills: %v", err)
	}

	// when
	result := session.CheckSkills(baseDir)

	// then
	if result.Status != domain.CheckOK {
		t.Errorf("expected CheckOK, got %v: %s", result.Status, result.Message)
	}
	if result.Name != "Skills" {
		t.Errorf("expected name 'Skills', got %q", result.Name)
	}
}

func TestCheckSkills_Missing(t *testing.T) {
	// given: empty dir (no skills installed)
	baseDir := t.TempDir()

	// when
	result := session.CheckSkills(baseDir)

	// then
	if result.Status != domain.CheckFail {
		t.Errorf("expected CheckFail for missing SKILL.md files, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckSkills_MissingSchemaVersion(t *testing.T) {
	// given: SKILL.md files exist but lack dmail-schema-version
	baseDir := t.TempDir()
	skillsDir := filepath.Join(baseDir, ".siren", "skills")
	os.MkdirAll(filepath.Join(skillsDir, "dmail-sendable"), 0755)
	os.MkdirAll(filepath.Join(skillsDir, "dmail-readable"), 0755)
	os.WriteFile(filepath.Join(skillsDir, "dmail-sendable", "SKILL.md"), []byte("---\nname: dmail-sendable\n---\n"), 0644)
	os.WriteFile(filepath.Join(skillsDir, "dmail-readable", "SKILL.md"), []byte("---\nname: dmail-readable\n---\n"), 0644)

	// when
	result := session.CheckSkills(baseDir)

	// then
	if result.Status != domain.CheckFail {
		t.Errorf("expected CheckFail for missing schema version, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckSkills_DeprecatedFeedbackKind(t *testing.T) {
	// given — SKILL.md with deprecated "kind: feedback" (pre-split)
	dir := t.TempDir()
	skillDir := filepath.Join(dir, ".siren", "skills", "dmail-readable")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"),
		[]byte("---\nname: dmail-readable\nmetadata:\n  dmail-schema-version: \"1\"\nconsumes:\n    - kind: feedback\n---\n"), 0644)
	// Also create sendable so it doesn't fail on missing
	sendDir := filepath.Join(dir, ".siren", "skills", "dmail-sendable")
	os.MkdirAll(sendDir, 0755)
	os.WriteFile(filepath.Join(sendDir, "SKILL.md"),
		[]byte("---\nname: dmail-sendable\nmetadata:\n  dmail-schema-version: \"1\"\nproduces:\n    - kind: specification\n---\n"), 0644)

	// when
	result := session.CheckSkills(dir)

	// then
	if result.Status != domain.CheckFail {
		t.Errorf("expected CheckFail for deprecated kind, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Hint, "init --force") {
		t.Errorf("hint should suggest init --force, got %q", result.Hint)
	}
}

func TestCheckSkills_UpdatedFeedbackKind(t *testing.T) {
	// given — SKILL.md with updated "kind: design-feedback" (post-split)
	dir := t.TempDir()
	skillDir := filepath.Join(dir, ".siren", "skills", "dmail-readable")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"),
		[]byte("---\nname: dmail-readable\nmetadata:\n  dmail-schema-version: \"1\"\nconsumes:\n    - kind: design-feedback\n---\n"), 0644)
	sendDir := filepath.Join(dir, ".siren", "skills", "dmail-sendable")
	os.MkdirAll(sendDir, 0755)
	os.WriteFile(filepath.Join(sendDir, "SKILL.md"),
		[]byte("---\nname: dmail-sendable\nmetadata:\n  dmail-schema-version: \"1\"\nproduces:\n    - kind: specification\n---\n"), 0644)

	// when
	result := session.CheckSkills(dir)

	// then
	if result.Status != domain.CheckOK {
		t.Errorf("expected CheckOK for updated kind, got %v: %s", result.Status, result.Message)
	}
}

func TestRunDoctor_ConfigFailure_ClaudeAuthAndMCPSkipped(t *testing.T) {
	// given: nonexistent config path → config check fails, cfg=nil
	// No mock needed: config failure causes auth/MCP to skip regardless of claude binary.
	dir := t.TempDir()
	ctx := context.Background()

	// when
	results := session.RunDoctor(ctx, "/nonexistent/sightjack.yaml", dir, platform.NewLogger(io.Discard, false), false, domain.ModeWave)

	// then: wave mode has 13 results (includes gh check)
	if len(results) != 15 {
		t.Fatalf("expected 15 results, got %d", len(results))
	}
	// Validate by name to avoid index-dependent assertions
	checkByName := func(name string) *domain.DoctorCheck {
		for i := range results {
			if results[i].Name == name {
				return &results[i]
			}
		}
		return nil
	}
	// Config should fail (nonexistent path)
	if c := checkByName("Config"); c == nil {
		t.Error("Config check not found")
	} else if c.Status != domain.CheckFail {
		t.Errorf("Config: expected FAIL, got %v", c.Status)
	}
	// claude-auth should be skipped (nil config)
	if c := checkByName("claude-auth"); c == nil {
		t.Error("claude-auth check not found")
	} else if c.Status != domain.CheckSkip {
		t.Errorf("claude-auth: expected SKIP, got %v: %s", c.Status, c.Message)
	}
	// linear-mcp should be skipped (nil config)
	if c := checkByName("linear-mcp"); c == nil {
		t.Error("linear-mcp check not found")
	} else if c.Status != domain.CheckSkip {
		t.Errorf("linear-mcp: expected SKIP, got %v: %s", c.Status, c.Message)
	}
	// claude-inference should be skipped (nil config)
	if c := checkByName("claude-inference"); c == nil {
		t.Error("claude-inference check not found")
	} else if c.Status != domain.CheckSkip {
		t.Errorf("claude-inference: expected SKIP, got %v: %s", c.Status, c.Message)
	}
	// context-budget should be skipped (nil config)
	if c := checkByName("context-budget"); c == nil {
		t.Error("context-budget check not found")
	} else if c.Status != domain.CheckSkip {
		t.Errorf("context-budget: expected SKIP, got %v: %s", c.Status, c.Message)
	}
}

func TestRunDoctor_ClaudeUnavailable_AuthAndMCPSkipped(t *testing.T) {
	// given: claude binary does not exist → Claude Auth + Linear MCP should be skipped
	// No mock needed: nonexistent claude_cmd causes CheckTool to fail, auth/MCP skip.
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "sightjack.yaml")
	os.WriteFile(cfgPath, []byte(`
tracker:
  team: "Test"
  project: "Project"
claude_cmd: "nonexistent-claude-binary-xyz"
`), 0644)

	ctx := context.Background()

	// when
	results := session.RunDoctor(ctx, cfgPath, dir, platform.NewLogger(io.Discard, false), false, domain.ModeWave)

	// then: wave mode has 13 results (includes gh check)
	if len(results) != 15 {
		t.Fatalf("expected 15 results, got %d", len(results))
	}
	// Validate by name to avoid index-dependent assertions
	checkByName := func(name string) *domain.DoctorCheck {
		for i := range results {
			if results[i].Name == name {
				return &results[i]
			}
		}
		return nil
	}
	// Config should pass
	if c := checkByName("Config"); c == nil {
		t.Error("Config check not found")
	} else if c.Status != domain.CheckOK {
		t.Errorf("Config: expected OK, got %v", c.Status)
	}
	// claude-auth should be skipped (claude binary unavailable)
	if c := checkByName("claude-auth"); c == nil {
		t.Error("claude-auth check not found")
	} else if c.Status != domain.CheckSkip {
		t.Errorf("claude-auth: expected SKIP, got %v: %s", c.Status, c.Message)
	}
	// linear-mcp should be skipped (wave mode)
	if c := checkByName("linear-mcp"); c == nil {
		t.Error("linear-mcp check not found")
	} else if c.Status != domain.CheckSkip {
		t.Errorf("linear-mcp: expected SKIP, got %v: %s", c.Status, c.Message)
	}
	// claude-inference should be skipped
	if c := checkByName("claude-inference"); c == nil {
		t.Error("claude-inference check not found")
	} else if c.Status != domain.CheckSkip {
		t.Errorf("claude-inference: expected SKIP, got %v: %s", c.Status, c.Message)
	}
	// context-budget should be skipped
	if c := checkByName("context-budget"); c == nil {
		t.Error("context-budget check not found")
	} else if c.Status != domain.CheckSkip {
		t.Errorf("context-budget: expected SKIP, got %v: %s", c.Status, c.Message)
	}
}

func TestRunDoctor_ReturnsAllResults(t *testing.T) {
	// given: fake-claude binary via config claude_cmd
	fakeClaude := buildFakeClaude(t)

	dir := t.TempDir()
	// Create .siren/ so CheckStateDir reports OK without repair
	os.MkdirAll(filepath.Join(dir, ".siren"), 0755)
	cfgPath := filepath.Join(dir, "sightjack.yaml")
	os.WriteFile(cfgPath, []byte("tracker:\n  team: \"Test\"\n  project: \"Project\"\nclaude_cmd: \""+fakeClaude+"\"\n"), 0644)

	ctx := context.Background()

	// when
	results := session.RunDoctor(ctx, cfgPath, dir, platform.NewLogger(io.Discard, false), false, domain.ModeLinear)

	// then: should have 13 results (git, claude, state dir, config, skills, event store, dead-letters, claude auth, linear mcp, claude-inference, context-budget, success-rate, skills-ref)
	if len(results) != 13 {
		t.Fatalf("expected 13 results, got %d: %v", len(results), results)
	}
	// git binary check (index 0)
	if results[0].Name != "git" || results[0].Status != domain.CheckOK {
		t.Errorf("git check: expected OK, got %v: %s", results[0].Status, results[0].Message)
	}
	// claude binary check should pass (fake-claude supports --version, index 1)
	if results[1].Status != domain.CheckOK {
		t.Errorf("claude check: expected OK, got %v: %s", results[1].Status, results[1].Message)
	}
	// State Dir (index 2)
	if results[2].Name != "State Dir" || results[2].Status != domain.CheckOK {
		t.Errorf("State Dir check: expected OK, got %v: %s", results[2].Status, results[2].Message)
	}
	// Config (index 3)
	if results[3].Name != "Config" || results[3].Status != domain.CheckOK {
		t.Errorf("Config check: expected OK, got %v: %s", results[3].Status, results[3].Message)
	}
	// Event Store (index 5)
	if results[5].Name != "Event Store" {
		t.Errorf("expected 'Event Store', got %q", results[5].Name)
	}
	// dead-letters (index 6)
	if results[6].Name != "dead-letters" {
		t.Errorf("expected 'dead-letters', got %q", results[6].Name)
	}
	// claude-auth should be OK (fake-claude mcp list succeeds, index 7)
	if results[7].Name != "claude-auth" {
		t.Errorf("expected 'claude-auth', got %q", results[7].Name)
	}
	if results[7].Status != domain.CheckOK {
		t.Errorf("claude-auth: expected OK, got %v: %s", results[7].Status, results[7].Message)
	}
	// linear-mcp should be OK (fake-claude outputs "linear ✓ connected", index 8)
	if results[8].Name != "linear-mcp" {
		t.Errorf("expected 'linear-mcp', got %q", results[8].Name)
	}
	if results[8].Status != domain.CheckOK {
		t.Errorf("linear-mcp: expected OK, got %v: %s", results[8].Status, results[8].Message)
	}
	// claude-inference should be OK (fake-claude --print responds with "2", index 9)
	if results[9].Name != "claude-inference" {
		t.Errorf("expected 'claude-inference', got %q", results[9].Name)
	}
	if results[9].Status != domain.CheckOK {
		t.Errorf("claude-inference: expected OK, got %v: %s", results[9].Status, results[9].Message)
	}
	// context-budget should be OK (index 10)
	if results[10].Name != "context-budget" {
		t.Errorf("expected 'context-budget', got %q", results[10].Name)
	}
	if results[10].Status != domain.CheckOK {
		t.Errorf("context-budget: expected OK, got %v: %s", results[10].Status, results[10].Message)
	}
	// success-rate (index 11)
	sr := results[11]
	if sr.Name != "success-rate" {
		t.Errorf("expected 'success-rate', got %q", sr.Name)
	}
	if sr.Status != domain.CheckOK {
		t.Errorf("success-rate: expected OK, got %v: %s", sr.Status, sr.Message)
	}
	if sr.Message != "no events" {
		t.Errorf("success-rate: expected 'no events', got %q", sr.Message)
	}
}

// --- CheckEventStore tests ---

func TestCheckEventStore_Valid(t *testing.T) {
	// given: valid JSONL event file in a session subdirectory
	dir := t.TempDir()
	sessionDir := filepath.Join(dir, ".siren", "events", "test-session-001")
	os.MkdirAll(sessionDir, 0755)
	os.WriteFile(filepath.Join(sessionDir, "2026-03-09.jsonl"),
		[]byte(`{"type":"wave.applied","timestamp":"2026-03-09T00:00:00Z"}`+"\n"), 0644)

	// when
	check := session.CheckEventStore(dir)

	// then
	if check.Status != domain.CheckOK {
		t.Errorf("expected OK, got %v: %s", check.Status, check.Message)
	}
	if check.Name != "Event Store" {
		t.Errorf("expected name 'Event Store', got %q", check.Name)
	}
	if !strings.Contains(check.Message, "1 session") && !strings.Contains(check.Message, "1 event") {
		t.Errorf("expected message to mention sessions and events, got: %s", check.Message)
	}
}

func TestCheckEventStore_Corrupt(t *testing.T) {
	// given: corrupt JSONL in a session subdirectory
	dir := t.TempDir()
	sessionDir := filepath.Join(dir, ".siren", "events", "test-session-001")
	os.MkdirAll(sessionDir, 0755)
	os.WriteFile(filepath.Join(sessionDir, "bad.jsonl"), []byte("not json\n"), 0644)

	// when
	check := session.CheckEventStore(dir)

	// then: corrupt lines produce WARN (not FAIL) because replay skips them
	if check.Status != domain.CheckWarn {
		t.Errorf("expected WARN, got %v: %s", check.Status, check.Message)
	}
	if !strings.Contains(check.Message, "corrupt") {
		t.Errorf("expected 'corrupt' in message: %s", check.Message)
	}
}

func TestCheckEventStore_NoDir(t *testing.T) {
	// given: no events directory at all
	dir := t.TempDir()

	// when
	check := session.CheckEventStore(dir)

	// then
	if check.Status != domain.CheckSkip {
		t.Errorf("expected SKIP, got %v: %s", check.Status, check.Message)
	}
}

func TestCheckEventStore_NoSessions(t *testing.T) {
	// given: events directory exists but no session subdirectories
	dir := t.TempDir()
	eventsDir := filepath.Join(dir, ".siren", "events")
	os.MkdirAll(eventsDir, 0755)

	// when
	check := session.CheckEventStore(dir)

	// then
	if check.Status != domain.CheckOK {
		t.Errorf("expected OK for empty events, got %v: %s", check.Status, check.Message)
	}
}

func TestCheckEventStore_MultipleSessions(t *testing.T) {
	// given: two sessions with valid events
	dir := t.TempDir()
	for _, sid := range []string{"session-001", "session-002"} {
		sessionDir := filepath.Join(dir, ".siren", "events", sid)
		os.MkdirAll(sessionDir, 0755)
		os.WriteFile(filepath.Join(sessionDir, "2026-03-09.jsonl"),
			[]byte(`{"type":"wave.applied","timestamp":"2026-03-09T00:00:00Z"}`+"\n"), 0644)
	}

	// when
	check := session.CheckEventStore(dir)

	// then
	if check.Status != domain.CheckOK {
		t.Errorf("expected OK, got %v: %s", check.Status, check.Message)
	}
	if !strings.Contains(check.Message, "2 session") {
		t.Errorf("expected message to mention 2 sessions, got: %s", check.Message)
	}
}

func TestRunDoctor_SuccessRateWithEvents(t *testing.T) {
	// given: fake-claude binary via config, valid config, and event data
	fakeClaude := buildFakeClaude(t)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "sightjack.yaml")
	os.WriteFile(cfgPath, []byte("tracker:\n  team: \"Test\"\n  project: \"Project\"\nclaude_cmd: \""+fakeClaude+"\"\n"), 0644)

	// Create event store with 2 applied and 1 rejected wave events
	sessionDir := filepath.Join(dir, ".siren", "events", "test-session-001")
	os.MkdirAll(sessionDir, 0755)
	events := strings.Join([]string{
		`{"id":"e1","type":"wave.applied","timestamp":"2026-03-01T10:00:00Z","data":{"wave_id":"w1","cluster_name":"c1","applied":1,"total_count":1},"session_id":"test-session-001"}`,
		`{"id":"e2","type":"wave.applied","timestamp":"2026-03-01T10:01:00Z","data":{"wave_id":"w2","cluster_name":"c1","applied":1,"total_count":1},"session_id":"test-session-001"}`,
		`{"id":"e3","type":"wave.rejected","timestamp":"2026-03-01T10:02:00Z","data":{"wave_id":"w3","cluster_name":"c1"},"session_id":"test-session-001"}`,
	}, "\n") + "\n"
	os.WriteFile(filepath.Join(sessionDir, "2026-03-01.jsonl"), []byte(events), 0644)

	ctx := context.Background()

	// when
	results := session.RunDoctor(ctx, cfgPath, dir, platform.NewLogger(io.Discard, false), false, domain.ModeWave)

	// then: find success-rate result
	var found bool
	for _, r := range results {
		if r.Name == "success-rate" {
			found = true
			if r.Status != domain.CheckOK {
				t.Errorf("success-rate: expected OK, got %v", r.Status)
			}
			want := "66.7% (2/3)"
			if r.Message != want {
				t.Errorf("success-rate: got %q, want %q", r.Message, want)
			}
			break
		}
	}
	if !found {
		t.Errorf("success-rate check not found in results (got %d results)", len(results))
	}
}

// --- Context Budget tests ---

func TestCheckContextBudget_LowUsage(t *testing.T) {
	t.Parallel()

	// given: stream-json with small init (2 tools, 1 skill)
	streamJSON := `{"type":"system","subtype":"init","model":"claude-opus-4-6","tools":["Read","Write"],"skills":["commit"],"plugins":[],"mcp_servers":[]}
{"type":"result","result":"2"}`

	// when
	result := session.CheckContextBudget(streamJSON, "")

	// then: should be OK (well under threshold)
	if result.Status != domain.CheckOK {
		t.Errorf("expected OK, got %v: %s", result.Status, result.Message)
	}
	if result.Hint != "" {
		t.Errorf("expected no hint for low usage, got %q", result.Hint)
	}
}

func TestCheckContextBudget_HighUsage(t *testing.T) {
	t.Parallel()

	// given: stream-json with many tools/skills/plugins + large hook output
	initLine := `{"type":"system","subtype":"init","model":"claude-opus-4-6","tools":["Read","Write","Edit","Grep","Glob","Bash","Agent"],"skills":["commit","review","test","deploy","lint","format","debug"],"plugins":[{"name":"p1"},{"name":"p2"},{"name":"p3"},{"name":"p4"},{"name":"p5"}],"mcp_servers":[{"name":"linear","status":"connected"},{"name":"filesystem","status":"connected"},{"name":"github","status":"connected"}]}`
	largeStdout := strings.Repeat("x", 100000)
	hookLine := fmt.Sprintf(`{"type":"system","subtype":"hook_response","hook_id":"h1","stdout":"%s","exit_code":0}`, largeStdout)
	resultLine := `{"type":"result","result":"2"}`
	streamJSON := initLine + "\n" + hookLine + "\n" + resultLine

	// when
	result := session.CheckContextBudget(streamJSON, "")

	// then: should be WARN with hint (over threshold)
	if result.Status != domain.CheckWarn {
		t.Errorf("expected WARN, got %v: %s", result.Status, result.Message)
	}
	if result.Hint == "" {
		t.Error("expected non-empty hint for high usage")
	}
}

func TestCheckContextBudget_EmptyStream(t *testing.T) {
	t.Parallel()

	// given: empty stream
	result := session.CheckContextBudget("", "")

	// then: should be OK (nothing to measure)
	if result.Status != domain.CheckOK {
		t.Errorf("expected OK for empty stream, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckContextBudget_NoInitMessage(t *testing.T) {
	t.Parallel()

	// given: stream-json with no init message
	result := session.CheckContextBudget(`{"type":"result","result":"2"}`, "")

	// then: should be OK (no init = no overhead)
	if result.Status != domain.CheckOK {
		t.Errorf("expected OK for no-init stream, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckContextBudget_WarnWithBreakdown(t *testing.T) {
	t.Parallel()

	// given: stream with many skills (exceeds threshold)
	initMsg := `{"type":"system","subtype":"init","tools":["Read","Write"],"skills":["a","b","c","d","e","f","g","h","i","j","k","l","m","n","o","p","q","r","s","t","u","v","w","x","y","z","aa","ab","ac","ad","ae","af","ag","ah","ai","aj","ak","al","am","an"],"plugins":[{"name":"p1"},{"name":"p2"},{"name":"p3"},{"name":"p4"},{"name":"p5"}],"mcp_servers":[{"name":"linear","status":"connected"}]}`
	streamJSON := initMsg + "\n"

	// when
	result := session.CheckContextBudget(streamJSON, "")

	// then
	if result.Status != domain.CheckWarn {
		t.Errorf("expected WARN, got %v", result.Status.StatusLabel())
	}
	if !strings.Contains(result.Message, "skills") {
		t.Errorf("message should contain breakdown with 'skills', got: %s", result.Message)
	}
	if result.Hint == "" {
		t.Error("expected hint for threshold exceeded")
	}
}

func TestCheckContextBudget_WarnHintWithSettingsFile(t *testing.T) {
	t.Parallel()

	// given: project with .claude/settings.json
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".claude"), 0o755)
	os.WriteFile(filepath.Join(dir, ".claude", "settings.json"), []byte(`{}`), 0o644)

	initMsg := `{"type":"system","subtype":"init","skills":["a","b","c","d","e","f","g","h","i","j","k","l","m","n","o","p","q","r","s","t","u","v","w","x","y","z","aa","ab","ac","ad","ae","af","ag","ah","ai","aj","ak","al","am","an","ao","ap"]}`
	streamJSON := initMsg + "\n"

	// when
	result := session.CheckContextBudget(streamJSON, dir)

	// then
	if result.Status != domain.CheckWarn {
		t.Errorf("expected WARN, got %v", result.Status.StatusLabel())
	}
	if !strings.Contains(result.Hint, "見直して") {
		t.Errorf("hint should say review settings, got: %s", result.Hint)
	}
}

// --- ExtractStreamResult tests ---

func TestExtractStreamResult_WithResult(t *testing.T) {
	t.Parallel()

	// given: stream-json with result
	streamJSON := `{"type":"system","subtype":"init","session_id":"s1"}
{"type":"assistant","message":{"content":[{"type":"text","text":"2"}]}}
{"type":"result","result":"2"}`

	// when
	result := session.ExtractStreamResult(streamJSON)

	// then
	if result != "2" {
		t.Errorf("expected '2', got %q", result)
	}
}

func TestExtractStreamResult_Empty(t *testing.T) {
	t.Parallel()

	// given: empty stream
	result := session.ExtractStreamResult("")

	// then
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestExtractStreamResult_NoResult(t *testing.T) {
	t.Parallel()

	// given: stream-json with no result message
	streamJSON := `{"type":"system","subtype":"init","session_id":"s1"}`

	// when
	result := session.ExtractStreamResult(streamJSON)

	// then
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}
