package sightjack

import (
	"bytes"
	"embed"
	"fmt"
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
	OutputPath      string
	StrictnessLevel string
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

// RenderNextGenPrompt renders the next-gen wave generation prompt.
func RenderNextGenPrompt(lang string, data NextGenPromptData) (string, error) {
	name := fmt.Sprintf("prompts/templates/wave_nextgen_%s.md.tmpl", lang)
	return renderTemplate(name, data)
}
