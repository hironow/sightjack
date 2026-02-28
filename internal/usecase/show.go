package usecase

import (
	"fmt"
	"io"
	"time"

	sightjack "github.com/hironow/sightjack"
	"github.com/hironow/sightjack/internal/session"
)

// ShowFromState loads the latest session state and renders the matrix navigator.
// This is the READ MODEL path for the show command.
func ShowFromState(w io.Writer, baseDir string, logger *sightjack.Logger) error {
	state, _, err := session.LoadLatestState(baseDir)
	if err != nil {
		logger.Info("Run 'sightjack scan' first.")
		return fmt.Errorf("no previous scan found: %w", err)
	}

	result := &sightjack.ScanResult{
		Completeness: state.Completeness,
	}
	for _, c := range state.Clusters {
		result.Clusters = append(result.Clusters, sightjack.ClusterScanResult{
			Name:         c.Name,
			Completeness: c.Completeness,
			IssueCount:   c.IssueCount,
		})
		result.TotalIssues += c.IssueCount
	}

	waves := session.RestoreWaves(state.Waves)
	strictness := state.StrictnessLevel
	if strictness == "" {
		strictness = "fog"
	}
	adrCount := session.CountADRFiles(session.ADRDir(baseDir))
	nav := session.RenderMatrixNavigator(result, state.Project, waves, adrCount, (*time.Time)(nil), strictness, state.ShibitoCount)
	fmt.Fprintln(w)
	fmt.Fprint(w, nav)
	logger.Info("Last scanned: %s", state.LastScanned.Format("2006-01-02 15:04:05"))
	return nil
}
