package sightjack

import (
	"bytes"
	"embed"
	"fmt"
	"strings"
	"text/template"
)

//go:embed prompts/templates/*.tmpl
var promptFS embed.FS

// ClassifyPromptData holds template data for the classify prompt.
type ClassifyPromptData struct {
	TeamFilter      string
	ProjectFilter   string
	CycleFilter     string
	OutputPath      string
	StrictnessLevel string
	LabelsEnabled   bool
	LabelPrefix     string
}

// DeepScanPromptData holds template data for the deep scan prompt.
type DeepScanPromptData struct {
	ClusterName     string
	IssueIDs        string
	OutputPath      string
	StrictnessLevel string
}

// WaveGeneratePromptData holds template data for the wave generation prompt.
type WaveGeneratePromptData struct {
	ClusterName     string
	Completeness    string
	Issues          string
	Observations    string
	DoDSection      string
	OutputPath      string
	StrictnessLevel string
}

// WaveApplyPromptData holds template data for the wave apply prompt.
type WaveApplyPromptData struct {
	WaveID          string
	ClusterName     string
	Title           string
	Actions         string
	OutputPath      string
	StrictnessLevel string
	LabelsEnabled   bool
	LabelPrefix     string
}

// ReadyLabelPromptData holds template data for the ready label prompt.
type ReadyLabelPromptData struct {
	ReadyLabel    string
	ReadyIssueIDs string
}

// ScribeADRPromptData holds template data for the scribe ADR generation prompt.
type ScribeADRPromptData struct {
	ClusterName     string
	WaveTitle       string
	WaveActions     string
	Analysis        string
	Reasoning       string
	ADRNumber       string
	OutputPath      string
	StrictnessLevel string
	ExistingADRs    []ExistingADR
}

// ArchitectDiscussPromptData holds template data for the architect discussion prompt.
type ArchitectDiscussPromptData struct {
	ClusterName     string
	WaveTitle       string
	WaveActions     string
	Topic           string
	OutputPath      string
	StrictnessLevel string
}

// NextGenPromptData holds template data for post-completion wave generation.
type NextGenPromptData struct {
	ClusterName     string
	Completeness    string
	Issues          string
	CompletedWaves  string
	ExistingADRs    []ExistingADR
	RejectedActions string
	DoDSection      string
	OutputPath      string
	StrictnessLevel string
}

// MatchDoDTemplate finds a DoD template matching the cluster name.
// Matching uses case-insensitive prefix match. When multiple keys match,
// the longest matching prefix wins. Equal-length ties are broken by
// lexicographic comparison of lowercased keys for deterministic behavior.
func MatchDoDTemplate(templates map[string]DoDTemplate, clusterName string) (bool, string) {
	lower := strings.ToLower(clusterName)
	bestKey := ""
	bestKeyLower := ""
	for key := range templates {
		keyLower := strings.ToLower(key)
		if !strings.HasPrefix(lower, keyLower) {
			continue
		}
		if bestKey == "" || len(keyLower) > len(bestKeyLower) ||
			(len(keyLower) == len(bestKeyLower) && keyLower < bestKeyLower) {
			bestKey = key
			bestKeyLower = keyLower
		}
	}
	if bestKey == "" {
		return false, ""
	}
	return true, bestKey
}

// FormatDoDSection formats a DoD template into a text section for prompt injection.
func FormatDoDSection(tmpl DoDTemplate) string {
	if len(tmpl.Must) == 0 && len(tmpl.Should) == 0 {
		return ""
	}
	var b strings.Builder
	if len(tmpl.Must) > 0 {
		b.WriteString("Must:\n")
		for _, item := range tmpl.Must {
			b.WriteString("- " + item + "\n")
		}
	}
	if len(tmpl.Should) > 0 {
		b.WriteString("Should:\n")
		for _, item := range tmpl.Should {
			b.WriteString("- " + item + "\n")
		}
	}
	return b.String()
}

func renderTemplate(name string, data any) (string, error) {
	tmpl, err := template.ParseFS(promptFS, name)
	if err != nil {
		return "", fmt.Errorf("parse template %s: %w", name, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template %s: %w", name, err)
	}
	return buf.String(), nil
}

// RenderClassifyPrompt renders the cluster classification prompt for the given language.
func RenderClassifyPrompt(lang string, data ClassifyPromptData) (string, error) {
	name := fmt.Sprintf("prompts/templates/scanner_classify_%s.md.tmpl", lang)
	return renderTemplate(name, data)
}

// RenderDeepScanPrompt renders the deep scan prompt for the given language.
func RenderDeepScanPrompt(lang string, data DeepScanPromptData) (string, error) {
	name := fmt.Sprintf("prompts/templates/scanner_deepscan_%s.md.tmpl", lang)
	return renderTemplate(name, data)
}

// RenderWaveGeneratePrompt renders the wave generation prompt for the given language.
func RenderWaveGeneratePrompt(lang string, data WaveGeneratePromptData) (string, error) {
	name := fmt.Sprintf("prompts/templates/wave_generate_%s.md.tmpl", lang)
	return renderTemplate(name, data)
}

// RenderWaveApplyPrompt renders the wave apply prompt for the given language.
func RenderWaveApplyPrompt(lang string, data WaveApplyPromptData) (string, error) {
	name := fmt.Sprintf("prompts/templates/wave_apply_%s.md.tmpl", lang)
	return renderTemplate(name, data)
}

// RenderScribeADRPrompt renders the scribe ADR generation prompt for the given language.
func RenderScribeADRPrompt(lang string, data ScribeADRPromptData) (string, error) {
	name := fmt.Sprintf("prompts/templates/scribe_adr_%s.md.tmpl", lang)
	return renderTemplate(name, data)
}

// RenderArchitectDiscussPrompt renders the architect discussion prompt for the given language.
func RenderArchitectDiscussPrompt(lang string, data ArchitectDiscussPromptData) (string, error) {
	name := fmt.Sprintf("prompts/templates/architect_discuss_%s.md.tmpl", lang)
	return renderTemplate(name, data)
}

// RenderReadyLabelPrompt renders the ready label prompt for the given language.
func RenderReadyLabelPrompt(lang string, data ReadyLabelPromptData) (string, error) {
	name := fmt.Sprintf("prompts/templates/ready_label_%s.md.tmpl", lang)
	return renderTemplate(name, data)
}

// RenderNextGenPrompt renders the next-gen wave generation prompt.
func RenderNextGenPrompt(lang string, data NextGenPromptData) (string, error) {
	name := fmt.Sprintf("prompts/templates/wave_nextgen_%s.md.tmpl", lang)
	return renderTemplate(name, data)
}
