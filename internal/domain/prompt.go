package domain

import (
	"strings"
)

// ClassifyPromptData holds template data for the classify prompt.
type ClassifyPromptData struct {
	TeamFilter      string
	ProjectFilter   string
	CycleFilter     string
	OutputPath      string
	StrictnessLevel string
	LabelsEnabled   bool
	LabelPrefix     string
	IsWaveMode      bool
}

// DeepScanPromptData holds template data for the deep scan prompt.
type DeepScanPromptData struct {
	ClusterName     string
	IssueIDs        string
	OutputPath      string
	StrictnessLevel string
	IsWaveMode      bool
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
	DoDSection      string
	OutputPath      string
	StrictnessLevel string
	LabelsEnabled   bool
	LabelPrefix     string
	IsWaveMode      bool
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
	FeedbackSection string
	ReportSection   string
	DoDSection      string
	OutputPath      string
	StrictnessLevel string
}

// AutoDiscussArchitectPromptData holds template data for the auto-discuss architect prompt.
type AutoDiscussArchitectPromptData struct {
	ClusterName     string
	WaveTitle       string
	WaveActions     string
	PriorContent    string // prior Devil's Advocate remarks (empty for round 0)
	FeedbackSection string // design-feedback D-Mails
	OutputPath      string
	StrictnessLevel string
}

// AutoDiscussDevilsAdvocatePromptData holds template data for the Devil's Advocate prompt.
type AutoDiscussDevilsAdvocatePromptData struct {
	ClusterName     string
	WaveTitle       string
	WaveActions     string
	PriorContent    string // prior Architect remarks
	ExistingADRs    []ExistingADR
	CLAUDEMDContent string // CLAUDE.md content (may be empty)
	OutputPath      string
	StrictnessLevel string
	RoundIndex      int
	TotalRounds     int
	IsFinalRound    bool
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
			(len(keyLower) == len(bestKeyLower) &&
				(keyLower < bestKeyLower || (keyLower == bestKeyLower && key < bestKey))) {
			bestKey = key
			bestKeyLower = keyLower
		}
	}
	if bestKey == "" {
		return false, ""
	}
	return true, bestKey
}

// ResolveDoDSection looks up a DoD template matching clusterName and formats
// it as a text section. Returns "" when no matching template is found or when
// the templates map is nil. This consolidates the MatchDoDTemplate + FormatDoDSection
// pattern used in scanner, wave_generator, and wave.
func ResolveDoDSection(templates map[string]DoDTemplate, clusterName string) string {
	if templates == nil {
		return ""
	}
	matched, key := MatchDoDTemplate(templates, clusterName)
	if !matched {
		return ""
	}
	return FormatDoDSection(templates[key])
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
