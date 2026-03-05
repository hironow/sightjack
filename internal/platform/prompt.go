package platform

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"

	"github.com/hironow/sightjack/internal/domain"
)

//go:embed templates/*.tmpl
var promptFS embed.FS

func renderPromptTemplate(name string, data any) (string, error) {
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
func RenderClassifyPrompt(lang string, data domain.ClassifyPromptData) (string, error) {
	name := fmt.Sprintf("templates/scanner_classify_%s.md.tmpl", lang)
	return renderPromptTemplate(name, data)
}

// RenderDeepScanPrompt renders the deep scan prompt for the given language.
func RenderDeepScanPrompt(lang string, data domain.DeepScanPromptData) (string, error) {
	name := fmt.Sprintf("templates/scanner_deepscan_%s.md.tmpl", lang)
	return renderPromptTemplate(name, data)
}

// RenderWaveGeneratePrompt renders the wave generation prompt for the given language.
func RenderWaveGeneratePrompt(lang string, data domain.WaveGeneratePromptData) (string, error) {
	name := fmt.Sprintf("templates/wave_generate_%s.md.tmpl", lang)
	return renderPromptTemplate(name, data)
}

// RenderWaveApplyPrompt renders the wave apply prompt for the given language.
func RenderWaveApplyPrompt(lang string, data domain.WaveApplyPromptData) (string, error) {
	name := fmt.Sprintf("templates/wave_apply_%s.md.tmpl", lang)
	return renderPromptTemplate(name, data)
}

// RenderScribeADRPrompt renders the scribe ADR generation prompt for the given language.
func RenderScribeADRPrompt(lang string, data domain.ScribeADRPromptData) (string, error) {
	name := fmt.Sprintf("templates/scribe_adr_%s.md.tmpl", lang)
	return renderPromptTemplate(name, data)
}

// RenderArchitectDiscussPrompt renders the architect discussion prompt for the given language.
func RenderArchitectDiscussPrompt(lang string, data domain.ArchitectDiscussPromptData) (string, error) {
	name := fmt.Sprintf("templates/architect_discuss_%s.md.tmpl", lang)
	return renderPromptTemplate(name, data)
}

// RenderReadyLabelPrompt renders the ready label prompt for the given language.
func RenderReadyLabelPrompt(lang string, data domain.ReadyLabelPromptData) (string, error) {
	name := fmt.Sprintf("templates/ready_label_%s.md.tmpl", lang)
	return renderPromptTemplate(name, data)
}

// RenderNextGenPrompt renders the next-gen wave generation prompt.
func RenderNextGenPrompt(lang string, data domain.NextGenPromptData) (string, error) {
	name := fmt.Sprintf("templates/wave_nextgen_%s.md.tmpl", lang)
	return renderPromptTemplate(name, data)
}
