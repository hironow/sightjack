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
	TeamFilter    string
	ProjectFilter string
	CycleFilter   string
	OutputPath    string
}

// DeepScanPromptData holds template data for the deep scan prompt.
type DeepScanPromptData struct {
	ClusterName string
	IssueIDs    string
	OutputPath  string
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
