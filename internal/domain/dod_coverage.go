package domain

// DoDCoverageReport summarises which clusters have a matching DoD template and
// which are uncovered.
type DoDCoverageReport struct {
	TotalClusters     int
	CoveredClusters   int
	UncoveredClusters []string
}

// BuildDoDCoverageReport constructs a DoDCoverageReport by matching each
// cluster name against the provided DoD templates using MatchDoDTemplate.
// Clusters without a matching template are listed in UncoveredClusters.
func BuildDoDCoverageReport(templates map[string]DoDTemplate, clusterNames []string) DoDCoverageReport {
	report := DoDCoverageReport{
		TotalClusters: len(clusterNames),
	}
	for _, name := range clusterNames {
		matched, _ := MatchDoDTemplate(templates, name) // nosemgrep: error-handling.ignored-error-go,error-handling.ignored-error-short-go -- second return is matched template name (string), not an error [permanent]
		if matched {
			report.CoveredClusters++
		} else {
			report.UncoveredClusters = append(report.UncoveredClusters, name)
		}
	}
	return report
}
