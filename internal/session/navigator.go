package session

import (
	"fmt"
	"io"
	"strings"
	"time"

	sightjack "github.com/hironow/sightjack"
)

const (
	NavigatorWidth = 60
	maxClusterName = 20
	waveColumns    = 4

	matrixClusterCol = 20
	matrixWaveCol    = 8
	matrixCompCol    = 6
)

// RenderNavigator produces an ASCII Link Navigator display inspired by
// the PS2 game SIREN's sight-jack interface. It visualises cluster
// completeness in a fixed-width text matrix.
func RenderNavigator(result *sightjack.ScanResult, projectName string) string {
	var b strings.Builder

	completePct := int(result.Completeness * 100)

	border := strings.Repeat("=", NavigatorWidth)
	b.WriteString(fmt.Sprintf("+%s+\n", border))
	b.WriteString(fmt.Sprintf("|%s|\n", Center("SIGHTJACK - Link Navigator", NavigatorWidth)))
	projName := PadRight(Truncate(projectName, 20), 20)
	projRow := "  Project: " + projName + "  |  Completeness: " + fmt.Sprintf("%3d%%", completePct)
	b.WriteString("|" + PadRight(projRow, NavigatorWidth) + "|\n")
	b.WriteString(fmt.Sprintf("+%s+\n", border))

	b.WriteString(fmt.Sprintf("|%s|\n", strings.Repeat(" ", NavigatorWidth)))
	header := fmt.Sprintf("  %-*s", maxClusterName, "Cluster")
	for i := 1; i <= waveColumns; i++ {
		header += fmt.Sprintf("  W%d  ", i)
	}
	header += "  Comp."
	b.WriteString(fmt.Sprintf("| %-*s|\n", NavigatorWidth-1, header))

	separator := "  " + strings.Repeat("-", NavigatorWidth-4)
	b.WriteString(fmt.Sprintf("| %-*s|\n", NavigatorWidth-1, separator))

	for _, cluster := range result.Clusters {
		pct := int(cluster.Completeness * 100)
		name := PadRight(Truncate(cluster.Name, maxClusterName), maxClusterName)
		row := "  " + name
		for range waveColumns {
			row += "  []  "
		}
		row += fmt.Sprintf("  %3d%%", pct)
		b.WriteString("|" + PadRight(" "+row, NavigatorWidth) + "|\n")
	}

	b.WriteString(fmt.Sprintf("|%s|\n", strings.Repeat(" ", NavigatorWidth)))
	b.WriteString(fmt.Sprintf("+%s+\n", border))
	b.WriteString(fmt.Sprintf("| %-*s|\n", NavigatorWidth-1, " [] not generated  [=] available  [#] complete"))
	b.WriteString(fmt.Sprintf("| %-*s|\n", NavigatorWidth-1, " [x] locked (dependency)"))
	b.WriteString(fmt.Sprintf("+%s+\n", border))

	return b.String()
}

// RenderMatrixNavigator renders the Link Navigator as a Pure ASCII matrix grid.
// Wave status symbols: [ ] available  [x] locked  [=] completed  [?] unknown
func RenderMatrixNavigator(result *sightjack.ScanResult, projectName string, waves []sightjack.Wave, adrCount int, lastScanned *time.Time, strictnessLevel string, shibitoCount int) string {
	// Group waves by cluster name
	wavesByCluster := make(map[string][]sightjack.Wave)
	for _, w := range waves {
		wavesByCluster[w.ClusterName] = append(wavesByCluster[w.ClusterName], w)
	}

	var b strings.Builder

	// --- Header block (free-form, above the grid) ---
	b.WriteString("  SIGHTJACK - Link Navigator\n")
	b.WriteString(fmt.Sprintf("  Project: %s\n", projectName))
	if lastScanned != nil {
		b.WriteString(fmt.Sprintf("  Session: resumed (last scan: %s)\n", lastScanned.Format("2006-01-02 15:04")))
	}
	if shibitoCount > 0 {
		b.WriteString(fmt.Sprintf("  Shibito: %d\n", shibitoCount))
	}

	// --- Grid ---
	clusterDash := strings.Repeat("-", matrixClusterCol)
	waveDash := strings.Repeat("-", matrixWaveCol)
	compDash := strings.Repeat("-", matrixCompCol)

	// Top border
	b.WriteString("+" + clusterDash)
	for range waveColumns {
		b.WriteString("+" + waveDash)
	}
	b.WriteString("+" + compDash + "+\n")

	// Header row
	b.WriteString("| " + PadRight("Cluster", matrixClusterCol-2) + " ")
	for i := 1; i <= waveColumns; i++ {
		b.WriteString("| " + PadRight(fmt.Sprintf("W%d", i), matrixWaveCol-2) + " ")
	}
	b.WriteString("| " + PadRight("Comp", matrixCompCol-2) + " ")
	b.WriteString("|\n")

	// Separator
	b.WriteString("+" + clusterDash)
	for range waveColumns {
		b.WriteString("+" + waveDash)
	}
	b.WriteString("+" + compDash + "+\n")

	// Data rows
	for _, cluster := range result.Clusters {
		pct := int(cluster.Completeness * 100)
		name := Truncate(cluster.Name, matrixClusterCol-2)
		issueCount := cluster.NumIssues()
		label := fmt.Sprintf("%s (%d)", name, issueCount)
		if DisplayWidth(label) > matrixClusterCol-2 {
			label = Truncate(label, matrixClusterCol-2)
		}
		b.WriteString("| " + PadRight(label, matrixClusterCol-2) + " ")

		clusterWaves := wavesByCluster[cluster.Name]
		for i := 0; i < waveColumns; i++ {
			if i < len(clusterWaves) {
				sym := waveStatusSymbol3(clusterWaves[i].Status)
				b.WriteString("| " + Center(sym, matrixWaveCol-2) + " ")
			} else {
				b.WriteString("| " + strings.Repeat(" ", matrixWaveCol-2) + " ")
			}
		}

		compStr := fmt.Sprintf("%d%%", pct)
		b.WriteString("| " + PadRight(compStr, matrixCompCol-2) + " ")
		b.WriteString("|\n")
	}

	// Bottom border
	b.WriteString("+" + clusterDash)
	for range waveColumns {
		b.WriteString("+" + waveDash)
	}
	b.WriteString("+" + compDash + "+\n")

	// Legend
	b.WriteString("  [=] completed  [ ] available  [x] locked  [?] unknown\n")

	// --- Footer (progress bar + metadata) ---
	progressBar := RenderProgressBar(result.Completeness, 20)
	footer := fmt.Sprintf("  %s  |  ADR: %d  |  Strictness: %s\n", progressBar, adrCount, strictnessLevel)
	b.WriteString(footer)

	// Wave listing: show wave titles grouped by cluster
	for _, cluster := range result.Clusters {
		clusterWaves := wavesByCluster[cluster.Name]
		if len(clusterWaves) == 0 {
			continue
		}
		for i, w := range clusterWaves {
			line := fmt.Sprintf("  %s W%d: %s %s",
				cluster.Name, i+1, waveStatusSymbol3(w.Status), w.Title)
			b.WriteString(line + "\n")
		}
	}

	return b.String()
}

// waveStatusSymbol returns a 5-character cell for a wave's status.
func waveStatusSymbol(status string) string {
	switch status {
	case "available":
		return " [ ] "
	case "locked":
		return " [x] "
	case "completed":
		return " [=] "
	default:
		return " [?] "
	}
}

// waveStatusSymbol3 returns a compact 3-character symbol for a wave's status.
func waveStatusSymbol3(status string) string {
	switch status {
	case "available":
		return "[ ]"
	case "locked":
		return "[x]"
	case "completed":
		return "[=]"
	default:
		return "[?]"
	}
}

// RenderProgressBar produces an ASCII progress bar: [====....] NN%
func RenderProgressBar(current float64, width int) string {
	if current < 0 {
		current = 0
	}
	if current > 1 {
		current = 1
	}
	if width <= 0 {
		width = 20
	}
	filled := int(current * float64(width))
	bar := strings.Repeat("=", filled) + strings.Repeat(".", width-filled)
	return fmt.Sprintf("[%s] %d%%", bar, int(current*100))
}

// isWide returns true for East Asian wide characters that occupy
// two columns in a fixed-width terminal.
func isWide(r rune) bool {
	return (r >= 0x1100 && r <= 0x115F) ||
		(r >= 0x2E80 && r <= 0x303E) ||
		(r >= 0x3040 && r <= 0x33BF) ||
		(r >= 0x3400 && r <= 0x4DBF) ||
		(r >= 0x4E00 && r <= 0x9FFF) ||
		(r >= 0xAC00 && r <= 0xD7AF) ||
		(r >= 0xF900 && r <= 0xFAFF) ||
		(r >= 0xFE30 && r <= 0xFE6F) ||
		(r >= 0xFF01 && r <= 0xFF60) ||
		(r >= 0xFFE0 && r <= 0xFFE6) ||
		(r >= 0x20000 && r <= 0x2FFFF) ||
		(r >= 0x30000 && r <= 0x3FFFF)
}

func runeWidth(r rune) int {
	if isWide(r) {
		return 2
	}
	return 1
}

func DisplayWidth(s string) int {
	w := 0
	for _, r := range s {
		w += runeWidth(r)
	}
	return w
}

func PadRight(s string, width int) string {
	dw := DisplayWidth(s)
	if dw >= width {
		return s
	}
	return s + strings.Repeat(" ", width-dw)
}

func Center(s string, width int) string {
	dw := DisplayWidth(s)
	if dw >= width {
		return Truncate(s, width)
	}
	pad := (width - dw) / 2
	return strings.Repeat(" ", pad) + s + strings.Repeat(" ", width-dw-pad)
}

func Truncate(s string, maxWidth int) string {
	if DisplayWidth(s) <= maxWidth {
		return s
	}
	w := 0
	for i, r := range s {
		rw := runeWidth(r)
		if w+rw+1 > maxWidth {
			return s[:i] + "~"
		}
		w += rw
	}
	return s
}

// DisplayRippleEffects shows cross-cluster effects after a wave is applied.
func DisplayRippleEffects(w io.Writer, ripples []sightjack.Ripple) {
	if len(ripples) == 0 {
		return
	}
	fmt.Fprintln(w, "\n  Ripple effects:")
	for _, r := range ripples {
		fmt.Fprintf(w, "    -> %s: %s\n", r.ClusterName, r.Description)
	}
}

// DisplayArchitectResponse shows the architect's analysis and any wave modifications.
func DisplayArchitectResponse(w io.Writer, resp *sightjack.ArchitectResponse) {
	fmt.Fprintf(w, "\n  [Architect] %s\n", resp.Analysis)
	if resp.Reasoning != "" {
		fmt.Fprintf(w, "\n  Reasoning: %s\n", resp.Reasoning)
	}
	if resp.ModifiedWave != nil {
		fmt.Fprintf(w, "\n  Modified actions (%d):\n", len(resp.ModifiedWave.Actions))
		for i, a := range resp.ModifiedWave.Actions {
			fmt.Fprintf(w, "    %d. [%s] %s: %s\n", i+1, a.Type, a.IssueID, a.Description)
		}
		fmt.Fprintf(w, "\n  Expected: %.0f%% -> %.0f%%\n",
			resp.ModifiedWave.Delta.Before*100, resp.ModifiedWave.Delta.After*100)
	}
}

// DisplayShibitoWarnings shows shibito resurrection detection warnings.
func DisplayShibitoWarnings(w io.Writer, warnings []sightjack.ShibitoWarning) {
	if len(warnings) == 0 {
		return
	}
	fmt.Fprintln(w, "\n  [Shibito] Resurrection warnings:")
	for _, warn := range warnings {
		fmt.Fprintf(w, "    %s -> %s [%s]: %s\n",
			warn.ClosedIssueID, warn.CurrentIssueID, warn.RiskLevel, warn.Description)
	}
}

// DisplayADRConflicts shows potential conflicts between new and existing ADRs.
func DisplayADRConflicts(w io.Writer, conflicts []sightjack.ADRConflict) {
	if len(conflicts) == 0 {
		return
	}
	for _, c := range conflicts {
		fmt.Fprintf(w, "  [Scribe] Warning: Potential conflict with ADR-%s: %s\n", c.ExistingADRID, c.Description)
	}
}

// DisplayCompletedWaveActions shows the actions that were applied in a completed wave.
func DisplayCompletedWaveActions(w io.Writer, wave sightjack.Wave) {
	fmt.Fprintf(w, "\n  --- %s - %s (completed) ---\n", wave.ClusterName, wave.Title)
	fmt.Fprintf(w, "  Actions applied (%d):\n", len(wave.Actions))
	for i, a := range wave.Actions {
		fmt.Fprintf(w, "    %d. [%s] %s: %s\n", i+1, a.Type, a.IssueID, a.Description)
	}
	if wave.Delta != (sightjack.WaveDelta{}) {
		fmt.Fprintf(w, "\n  Result: %.0f%% -> %.0f%%\n", wave.Delta.Before*100, wave.Delta.After*100)
	}
}

// DisplayWaveCompletion shows a rich wave completion summary with grouped ripple effects.
func DisplayWaveCompletion(w io.Writer, wave sightjack.Wave, ripples []sightjack.Ripple, overallCompleteness float64, newWavesAvailable int) {
	fmt.Fprintf(w, "\n  Wave completed: %s - %s\n", wave.ClusterName, wave.Title)
	fmt.Fprintf(w, "  Completeness: %.0f%% -> %.0f%%\n", wave.Delta.Before*100, wave.Delta.After*100)

	if len(ripples) > 0 {
		fmt.Fprintln(w, "\n  Ripple effects:")
		// Group by cluster
		grouped := make(map[string][]sightjack.Ripple)
		var order []string
		for _, r := range ripples {
			if _, seen := grouped[r.ClusterName]; !seen {
				order = append(order, r.ClusterName)
			}
			grouped[r.ClusterName] = append(grouped[r.ClusterName], r)
		}
		for _, name := range order {
			fmt.Fprintf(w, "    %s:\n", name)
			for _, r := range grouped[name] {
				fmt.Fprintf(w, "      -> %s\n", r.Description)
			}
		}
	}

	if newWavesAvailable > 0 {
		fmt.Fprintf(w, "\n  New waves available: %d\n", newWavesAvailable)
	}

	fmt.Fprintf(w, "  %s\n", RenderProgressBar(overallCompleteness, 20))
}

// DisplayScribeResponse shows the scribe's ADR generation result.
func DisplayScribeResponse(w io.Writer, resp *sightjack.ScribeResponse) {
	fmt.Fprintf(w, "\n  [Scribe] ADR %s: %s\n", resp.ADRID, resp.Title)
	fmt.Fprintf(w, "  Saved to %s/%s-%s.md\n", ADRSubdir, resp.ADRID, SanitizeADRTitle(resp.Title))
}
