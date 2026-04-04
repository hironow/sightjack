package platform

import (
	"fmt"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/harness/filter"
)

// RenderClassifyPrompt renders the cluster classification prompt for the given language.
func RenderClassifyPrompt(lang string, data domain.ClassifyPromptData) (string, error) {
	name := "scanner_classify_" + lang
	reg := filter.DefaultRegistry()
	result, err := reg.Expand(name, map[string]string{
		"mode_section":     filter.BuildClassifyModeSection(data.IsWaveMode, lang),
		"strictness_level": data.StrictnessLevel,
		"filter_section":   filter.BuildClassifyFilterSection(data.IsWaveMode, data.TeamFilter, data.ProjectFilter, data.CycleFilter, lang),
		"steps_section":    filter.BuildClassifyStepsSection(data.IsWaveMode, lang),
		"output_path":      data.OutputPath,
		"labels_note":      "",
	})
	if err != nil {
		return "", fmt.Errorf("expand classify prompt %s: %w", lang, err)
	}
	return result, nil
}

// RenderDeepScanPrompt renders the deep scan prompt for the given language.
func RenderDeepScanPrompt(lang string, data domain.DeepScanPromptData) (string, error) {
	name := "scanner_deepscan_" + lang
	reg := filter.DefaultRegistry()
	result, err := reg.Expand(name, map[string]string{
		"strictness_level": data.StrictnessLevel,
		"cluster_name":     data.ClusterName,
		"issue_ids":        data.IssueIDs,
		"output_path":      data.OutputPath,
		"mode_note":        filter.BuildDeepScanModeNote(data.IsWaveMode, lang),
	})
	if err != nil {
		return "", fmt.Errorf("expand deepscan prompt %s: %w", lang, err)
	}
	return result, nil
}

// RenderWaveGeneratePrompt renders the wave generation prompt for the given language.
func RenderWaveGeneratePrompt(lang string, data domain.WaveGeneratePromptData) (string, error) {
	name := "wave_generate_" + lang
	reg := filter.DefaultRegistry()
	result, err := reg.Expand(name, map[string]string{
		"strictness_level": data.StrictnessLevel,
		"cluster_name":     data.ClusterName,
		"completeness":     data.Completeness,
		"issues":           data.Issues,
		"observations":     data.Observations,
		"dod_section":      filter.BuildDoDSection(data.DoDSection, lang),
		"output_path":      data.OutputPath,
	})
	if err != nil {
		return "", fmt.Errorf("expand wave_generate prompt %s: %w", lang, err)
	}
	return result, nil
}

// RenderWaveApplyPrompt renders the wave apply prompt for the given language.
func RenderWaveApplyPrompt(lang string, data domain.WaveApplyPromptData) (string, error) {
	name := "wave_apply_" + lang
	reg := filter.DefaultRegistry()
	result, err := reg.Expand(name, map[string]string{
		"mode_intro":       filter.BuildWaveApplyModeIntro(data.IsWaveMode, lang),
		"strictness_level": data.StrictnessLevel,
		"wave_id":          data.WaveID,
		"cluster_name":     data.ClusterName,
		"title":            data.Title,
		"actions":          data.Actions,
		"steps_section":    filter.BuildWaveApplyStepsSection(data.IsWaveMode, lang),
		"dod_section":      filter.BuildDoDSection(data.DoDSection, lang),
		"output_path":      data.OutputPath,
	})
	if err != nil {
		return "", fmt.Errorf("expand wave_apply prompt %s: %w", lang, err)
	}
	return result, nil
}

// RenderScribeADRPrompt renders the scribe ADR generation prompt for the given language.
func RenderScribeADRPrompt(lang string, data domain.ScribeADRPromptData) (string, error) {
	name := "scribe_adr_" + lang
	reg := filter.DefaultRegistry()
	result, err := reg.Expand(name, map[string]string{
		"strictness_level":      data.StrictnessLevel,
		"cluster_name":          data.ClusterName,
		"wave_title":            data.WaveTitle,
		"wave_actions":          data.WaveActions,
		"analysis":              data.Analysis,
		"reasoning":             data.Reasoning,
		"adr_number":            data.ADRNumber,
		"existing_adrs_section": filter.BuildScribeExistingADRsSection(data.ExistingADRs, lang),
		"output_path":           data.OutputPath,
	})
	if err != nil {
		return "", fmt.Errorf("expand scribe_adr prompt %s: %w", lang, err)
	}
	return result, nil
}

// RenderArchitectDiscussPrompt renders the architect discussion prompt for the given language.
func RenderArchitectDiscussPrompt(lang string, data domain.ArchitectDiscussPromptData) (string, error) {
	name := "architect_discuss_" + lang
	reg := filter.DefaultRegistry()
	result, err := reg.Expand(name, map[string]string{
		"strictness_level": data.StrictnessLevel,
		"cluster_name":     data.ClusterName,
		"wave_title":       data.WaveTitle,
		"wave_actions":     data.WaveActions,
		"topic":            data.Topic,
		"output_path":      data.OutputPath,
	})
	if err != nil {
		return "", fmt.Errorf("expand architect_discuss prompt %s: %w", lang, err)
	}
	return result, nil
}

// RenderReadyLabelPrompt renders the ready label prompt for the given language.
func RenderReadyLabelPrompt(lang string, data domain.ReadyLabelPromptData) (string, error) {
	name := "ready_label_" + lang
	reg := filter.DefaultRegistry()
	result, err := reg.Expand(name, map[string]string{
		"ready_label":    data.ReadyLabel,
		"ready_issue_ids": data.ReadyIssueIDs,
	})
	if err != nil {
		return "", fmt.Errorf("expand ready_label prompt %s: %w", lang, err)
	}
	return result, nil
}

// RenderNextGenPrompt renders the next-gen wave generation prompt.
func RenderNextGenPrompt(lang string, data domain.NextGenPromptData) (string, error) {
	name := "wave_nextgen_" + lang
	reg := filter.DefaultRegistry()
	result, err := reg.Expand(name, map[string]string{
		"strictness_level": data.StrictnessLevel,
		"cluster_name":     data.ClusterName,
		"completeness":     data.Completeness,
		"issues":           data.Issues,
		"completed_waves":  data.CompletedWaves,
		"adrs_section":     filter.BuildExistingADRsSection(data.ExistingADRs, lang),
		"rejected_section": filter.BuildRejectedActionsSection(data.RejectedActions, lang),
		"feedback_section": filter.BuildFeedbackSection(data.FeedbackSection, lang),
		"report_section":   filter.BuildReportSection(data.ReportSection, lang),
		"dod_section":      filter.BuildDoDSection(data.DoDSection, lang),
		"output_path":      data.OutputPath,
	})
	if err != nil {
		return "", fmt.Errorf("expand wave_nextgen prompt %s: %w", lang, err)
	}
	return result, nil
}

// RenderAutoDiscussArchitectPrompt renders the auto-discuss architect prompt.
func RenderAutoDiscussArchitectPrompt(lang string, data domain.AutoDiscussArchitectPromptData) (string, error) {
	name := "auto_discuss_architect_" + lang
	reg := filter.DefaultRegistry()
	result, err := reg.Expand(name, map[string]string{
		"strictness_level":    data.StrictnessLevel,
		"cluster_name":        data.ClusterName,
		"wave_title":          data.WaveTitle,
		"wave_actions":        data.WaveActions,
		"feedback_section":    filter.BuildAutoDiscussFeedbackSection(data.FeedbackSection, lang),
		"prior_section":       filter.BuildAutoDiscussPriorSection(data.PriorContent, lang),
		"instructions_section": filter.BuildAutoDiscussInstructionsSection(data.PriorContent != "", lang),
		"output_path":         data.OutputPath,
	})
	if err != nil {
		return "", fmt.Errorf("expand auto_discuss_architect prompt %s: %w", lang, err)
	}
	return result, nil
}

// RenderAutoDiscussDevilsAdvocatePrompt renders the auto-discuss Devil's Advocate prompt.
func RenderAutoDiscussDevilsAdvocatePrompt(lang string, data domain.AutoDiscussDevilsAdvocatePromptData) (string, error) {
	name := "auto_discuss_devils_advocate_" + lang
	reg := filter.DefaultRegistry()
	result, err := reg.Expand(name, map[string]string{
		"strictness_level":        data.StrictnessLevel,
		"cluster_name":            data.ClusterName,
		"wave_title":              data.WaveTitle,
		"wave_actions":            data.WaveActions,
		"prior_content":           data.PriorContent,
		"adrs_section":            filter.BuildDAExistingADRsSection(data.ExistingADRs, lang),
		"claudemd_section":        filter.BuildDAClaudeMDSection(data.CLAUDEMDContent, lang),
		"round_info":              filter.BuildDARoundInfo(data.RoundIndex, data.TotalRounds, lang),
		"final_round_instructions": filter.BuildDAFinalRoundInstructions(data.IsFinalRound, lang),
		"output_format":           filter.BuildDAOutputFormat(data.IsFinalRound, lang),
		"output_path":             data.OutputPath,
	})
	if err != nil {
		return "", fmt.Errorf("expand auto_discuss_devils_advocate prompt %s: %w", lang, err)
	}
	return result, nil
}

