package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sightjack "github.com/hironow/sightjack"
)

func TestRunInit_CreatesConfigFile(t *testing.T) {
	// given: empty directory with stdin providing all 4 answers
	dir := t.TempDir()
	input := strings.NewReader("Engineering\nMy Project\nja\nalert\n")
	var output bytes.Buffer

	// when
	err := runInit(dir, input, &output)

	// then
	if err != nil {
		t.Fatalf("runInit failed: %v", err)
	}
	cfgPath := sightjack.ConfigPath(dir)
	data, readErr := os.ReadFile(cfgPath)
	if readErr != nil {
		t.Fatalf("config not created: %v", readErr)
	}
	content := string(data)
	if !strings.Contains(content, "Engineering") {
		t.Errorf("expected team in config, got:\n%s", content)
	}
	if !strings.Contains(content, "My Project") {
		t.Errorf("expected project in config, got:\n%s", content)
	}
}

func TestRunInit_DefaultValues(t *testing.T) {
	// given: empty lines for lang and strictness (use defaults)
	dir := t.TempDir()
	input := strings.NewReader("Team\nProject\n\n\n")
	var output bytes.Buffer

	// when
	err := runInit(dir, input, &output)

	// then
	if err != nil {
		t.Fatalf("runInit failed: %v", err)
	}
	data, _ := os.ReadFile(sightjack.ConfigPath(dir))
	content := string(data)
	if !strings.Contains(content, `lang: "ja"`) {
		t.Errorf("expected default lang 'ja' in config, got:\n%s", content)
	}
	if !strings.Contains(content, "default: fog") {
		t.Errorf("expected default strictness 'fog' in config, got:\n%s", content)
	}
}

func TestRunInit_ExistingConfigError(t *testing.T) {
	// given: directory with existing .siren/config.yaml
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".siren"), 0755)
	os.WriteFile(sightjack.ConfigPath(dir), []byte("existing"), 0644)
	input := strings.NewReader("Team\nProject\n\n\n")
	var output bytes.Buffer

	// when
	err := runInit(dir, input, &output)

	// then: should return error
	if err == nil {
		t.Fatal("expected error when config already exists")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' in error, got: %v", err)
	}
}

func TestRunInit_InvalidLang_RepromptsUntilValid(t *testing.T) {
	// given: first answer is invalid "jp", second is valid "en"
	dir := t.TempDir()
	input := strings.NewReader("Team\nProject\njp\nen\n\n")
	var output bytes.Buffer

	// when
	err := runInit(dir, input, &output)

	// then: should succeed with the valid value
	if err != nil {
		t.Fatalf("runInit failed: %v", err)
	}
	data, _ := os.ReadFile(sightjack.ConfigPath(dir))
	content := string(data)
	if !strings.Contains(content, `lang: "en"`) {
		t.Errorf("expected lang 'en' in config, got:\n%s", content)
	}
	if !strings.Contains(output.String(), "invalid") {
		t.Errorf("expected 'invalid' error in output, got:\n%s", output.String())
	}
}

func TestRunInit_InvalidStrictness_RepromptsUntilValid(t *testing.T) {
	// given: first strictness answer is invalid "strict", second is valid "alert"
	dir := t.TempDir()
	input := strings.NewReader("Team\nProject\n\nstrict\nalert\n")
	var output bytes.Buffer

	// when
	err := runInit(dir, input, &output)

	// then: should succeed with the valid value
	if err != nil {
		t.Fatalf("runInit failed: %v", err)
	}
	data, _ := os.ReadFile(sightjack.ConfigPath(dir))
	content := string(data)
	if !strings.Contains(content, "default: alert") {
		t.Errorf("expected strictness 'alert' in config, got:\n%s", content)
	}
	if !strings.Contains(output.String(), "invalid") {
		t.Errorf("expected 'invalid' error in output, got:\n%s", output.String())
	}
}

func TestRunInit_EOFDuringTeam_ReturnsError(t *testing.T) {
	// given: stdin closes before team is entered
	dir := t.TempDir()
	input := strings.NewReader("")
	var output bytes.Buffer

	// when
	err := runInit(dir, input, &output)

	// then
	if err == nil {
		t.Fatal("expected error for EOF during team input")
	}
	if !strings.Contains(err.Error(), "unexpected end of input") {
		t.Errorf("expected 'unexpected end of input' error, got: %v", err)
	}
}

func TestRunInit_StrictnessCaseInsensitive(t *testing.T) {
	// given: mixed-case strictness values should be accepted
	for _, input := range []struct {
		value string
		want  string
	}{
		{"Alert", "alert"},
		{"LOCKDOWN", "lockdown"},
		{"Fog", "fog"},
	} {
		t.Run(input.value, func(t *testing.T) {
			dir := t.TempDir()
			r := strings.NewReader("Team\nProject\n\n" + input.value + "\n")
			var output bytes.Buffer

			// when
			err := runInit(dir, r, &output)

			// then
			if err != nil {
				t.Fatalf("runInit failed: %v", err)
			}
			data, _ := os.ReadFile(sightjack.ConfigPath(dir))
			content := string(data)
			if !strings.Contains(content, "default: "+input.want) {
				t.Errorf("expected strictness %q in config, got:\n%s", input.want, content)
			}
		})
	}
}

func TestRunInit_OutputContainsPrompts(t *testing.T) {
	// given
	dir := t.TempDir()
	input := strings.NewReader("Team\nProject\n\n\n")
	var output bytes.Buffer

	// when
	runInit(dir, input, &output)

	// then: output should contain the interactive prompts
	out := output.String()
	if !strings.Contains(out, "Linear team name") {
		t.Errorf("expected 'Linear team name' prompt in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Created .siren/config.yaml") {
		t.Errorf("expected success message in output, got:\n%s", out)
	}
}

func TestRunInit_CreatesGitIgnore(t *testing.T) {
	// given
	dir := t.TempDir()
	input := strings.NewReader("Team\nProject\n\n\n")
	var output bytes.Buffer

	// when
	err := runInit(dir, input, &output)

	// then: .siren/.gitignore should exist
	if err != nil {
		t.Fatalf("runInit failed: %v", err)
	}
	data, readErr := os.ReadFile(filepath.Join(dir, ".siren", ".gitignore"))
	if readErr != nil {
		t.Fatalf(".gitignore not created: %v", readErr)
	}
	content := string(data)
	if !strings.Contains(content, "events/") {
		t.Errorf("expected events/ in .gitignore, got:\n%s", content)
	}
}

func TestRunInit_InstallsSkills(t *testing.T) {
	// given
	dir := t.TempDir()
	input := strings.NewReader("Team\nProject\n\n\n")
	var output bytes.Buffer

	// when
	err := runInit(dir, input, &output)

	// then: skill files should be installed
	if err != nil {
		t.Fatalf("runInit failed: %v", err)
	}
	sendable, readErr := os.ReadFile(filepath.Join(dir, ".siren", "skills", "dmail-sendable", "SKILL.md"))
	if readErr != nil {
		t.Fatalf("dmail-sendable SKILL.md not installed: %v", readErr)
	}
	if !strings.Contains(string(sendable), "name: dmail-sendable") {
		t.Errorf("unexpected dmail-sendable content: %s", sendable)
	}
	readable, readErr := os.ReadFile(filepath.Join(dir, ".siren", "skills", "dmail-readable", "SKILL.md"))
	if readErr != nil {
		t.Fatalf("dmail-readable SKILL.md not installed: %v", readErr)
	}
	if !strings.Contains(string(readable), "name: dmail-readable") {
		t.Errorf("unexpected dmail-readable content: %s", readable)
	}
}

func TestRunInit_CreatesMailDirs(t *testing.T) {
	// given
	dir := t.TempDir()
	input := strings.NewReader("Team\nProject\n\n\n")
	var output bytes.Buffer

	// when
	err := runInit(dir, input, &output)

	// then: mail directories should be created
	if err != nil {
		t.Fatalf("runInit failed: %v", err)
	}
	for _, sub := range []string{"inbox", "outbox", "archive"} {
		path := filepath.Join(dir, ".siren", sub)
		info, statErr := os.Stat(path)
		if statErr != nil {
			t.Errorf("%s not created: %v", sub, statErr)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%s is not a directory", sub)
		}
	}
}
