package domain_test

import (
	"testing"

	"github.com/hironow/sightjack/internal/domain"
)

func TestDoDCoverageReport_NoClusters(t *testing.T) {
	// given: empty templates and no clusters
	templates := map[string]domain.DoDTemplate{}
	clusterNames := []string{}

	// when
	report := domain.BuildDoDCoverageReport(templates, clusterNames)

	// then
	if report.TotalClusters != 0 {
		t.Errorf("TotalClusters: expected 0, got %d", report.TotalClusters)
	}
	if report.CoveredClusters != 0 {
		t.Errorf("CoveredClusters: expected 0, got %d", report.CoveredClusters)
	}
	if len(report.UncoveredClusters) != 0 {
		t.Errorf("UncoveredClusters: expected empty, got %v", report.UncoveredClusters)
	}
}

func TestDoDCoverageReport_AllCovered(t *testing.T) {
	// given: templates covering all clusters
	templates := map[string]domain.DoDTemplate{
		"auth": {Must: []string{"tests pass"}, Should: []string{"lint passes"}},
		"api":  {Must: []string{"docs updated"}},
	}
	clusterNames := []string{"auth", "api"}

	// when
	report := domain.BuildDoDCoverageReport(templates, clusterNames)

	// then
	if report.TotalClusters != 2 {
		t.Errorf("TotalClusters: expected 2, got %d", report.TotalClusters)
	}
	if report.CoveredClusters != 2 {
		t.Errorf("CoveredClusters: expected 2, got %d", report.CoveredClusters)
	}
	if len(report.UncoveredClusters) != 0 {
		t.Errorf("UncoveredClusters: expected empty, got %v", report.UncoveredClusters)
	}
}

func TestDoDCoverageReport_SomeCovered(t *testing.T) {
	// given: template only for "auth", but clusters include "infra" and "billing"
	templates := map[string]domain.DoDTemplate{
		"auth": {Must: []string{"tests pass"}},
	}
	clusterNames := []string{"auth", "infra", "billing"}

	// when
	report := domain.BuildDoDCoverageReport(templates, clusterNames)

	// then
	if report.TotalClusters != 3 {
		t.Errorf("TotalClusters: expected 3, got %d", report.TotalClusters)
	}
	if report.CoveredClusters != 1 {
		t.Errorf("CoveredClusters: expected 1 (auth), got %d", report.CoveredClusters)
	}
	if len(report.UncoveredClusters) != 2 {
		t.Errorf("UncoveredClusters: expected 2 (infra, billing), got %v", report.UncoveredClusters)
	}
	for _, uc := range report.UncoveredClusters {
		if uc == "auth" {
			t.Error("UncoveredClusters: auth should not be in uncovered list")
		}
	}
}

func TestDoDCoverageReport_UsesMatchDoDTemplate(t *testing.T) {
	// given: template key is a prefix, cluster name is longer
	templates := map[string]domain.DoDTemplate{
		"auth": {Must: []string{"tests pass"}},
	}
	// "authentication" should match "auth" via MatchDoDTemplate prefix logic
	clusterNames := []string{"authentication", "billing"}

	// when
	report := domain.BuildDoDCoverageReport(templates, clusterNames)

	// then: "authentication" covered by prefix "auth"
	if report.CoveredClusters != 1 {
		t.Errorf("CoveredClusters: expected 1 (authentication via prefix auth), got %d", report.CoveredClusters)
	}
	if len(report.UncoveredClusters) != 1 || report.UncoveredClusters[0] != "billing" {
		t.Errorf("UncoveredClusters: expected [billing], got %v", report.UncoveredClusters)
	}
}
