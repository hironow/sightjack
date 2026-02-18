package sightjack

import (
	"fmt"
	"strings"
	"time"
)

const (
	navigatorWidth = 60
	maxClusterName = 20
	waveColumns    = 4

	matrixClusterCol = 20
	matrixWaveCol    = 8
	matrixCompCol    = 6
)

// RenderNavigator produces an ASCII Link Navigator display inspired by
// the PS2 game SIREN's sight-jack interface. It visualises cluster
// completeness in a fixed-width text matrix.
func RenderNavigator(result *ScanResult, projectName string) string {
	var b strings.Builder

	completePct := int(result.Completeness * 100)

	border := strings.Repeat("=", navigatorWidth)
	b.WriteString(fmt.Sprintf("+%s+\n", border))
	b.WriteString(fmt.Sprintf("|%s|\n", center("SIGHTJACK - Link Navigator", navigatorWidth)))
	projName := padRight(truncate(projectName, 20), 20)
	projRow := "  Project: " + projName + "  |  Completeness: " + fmt.Sprintf("%3d%%", completePct)
	b.WriteString("|" + padRight(projRow, navigatorWidth) + "|\n")
	b.WriteString(fmt.Sprintf("+%s+\n", border))

	b.WriteString(fmt.Sprintf("|%s|\n", strings.Repeat(" ", navigatorWidth)))
	header := fmt.Sprintf("  %-*s", maxClusterName, "Cluster")
	for i := 1; i <= waveColumns; i++ {
		header += fmt.Sprintf("  W%d  ", i)
	}
	header += "  Comp."
	b.WriteString(fmt.Sprintf("| %-*s|\n", navigatorWidth-1, header))

	separator := "  " + strings.Repeat("-", navigatorWidth-4)
	b.WriteString(fmt.Sprintf("| %-*s|\n", navigatorWidth-1, separator))

	for _, cluster := range result.Clusters {
		pct := int(cluster.Completeness * 100)
		name := padRight(truncate(cluster.Name, maxClusterName), maxClusterName)
		row := "  " + name
		for range waveColumns {
			row += "  []  "
		}
		row += fmt.Sprintf("  %3d%%", pct)
		b.WriteString("|" + padRight(" "+row, navigatorWidth) + "|\n")
	}

	b.WriteString(fmt.Sprintf("|%s|\n", strings.Repeat(" ", navigatorWidth)))
	b.WriteString(fmt.Sprintf("+%s+\n", border))
	b.WriteString(fmt.Sprintf("| %-*s|\n", navigatorWidth-1, " [] not generated  [=] available  [#] complete"))
	b.WriteString(fmt.Sprintf("| %-*s|\n", navigatorWidth-1, " [x] locked (dependency)"))
	b.WriteString(fmt.Sprintf("+%s+\n", border))

	return b.String()
}

// RenderMatrixNavigator renders the Link Navigator as a Pure ASCII matrix grid.
// Wave status symbols: [ ] available  [x] locked  [=] completed  [?] unknown
func RenderMatrixNavigator(result *ScanResult, projectName string, waves []Wave, adrCount int, lastScanned *time.Time, strictnessLevel string, shibitoCount int) string {
	// Group waves by cluster name
	wavesByCluster := make(map[string][]Wave)
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
	b.WriteString("| " + padRight("Cluster", matrixClusterCol-2) + " ")
	for i := 1; i <= waveColumns; i++ {
		b.WriteString("| " + padRight(fmt.Sprintf("W%d", i), matrixWaveCol-2) + " ")
	}
	b.WriteString("| " + padRight("Comp", matrixCompCol-2) + " ")
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
		name := truncate(cluster.Name, matrixClusterCol-2)
		issueCount := len(cluster.Issues)
		label := fmt.Sprintf("%s (%d)", name, issueCount)
		if displayWidth(label) > matrixClusterCol-2 {
			label = truncate(label, matrixClusterCol-2)
		}
		b.WriteString("| " + padRight(label, matrixClusterCol-2) + " ")

		clusterWaves := wavesByCluster[cluster.Name]
		for i := 0; i < waveColumns; i++ {
			if i < len(clusterWaves) {
				sym := waveStatusSymbol3(clusterWaves[i].Status)
				b.WriteString("| " + center(sym, matrixWaveCol-2) + " ")
			} else {
				b.WriteString("| " + strings.Repeat(" ", matrixWaveCol-2) + " ")
			}
		}

		compStr := fmt.Sprintf("%d%%", pct)
		b.WriteString("| " + padRight(compStr, matrixCompCol-2) + " ")
		b.WriteString("|\n")
	}

	// Bottom border
	b.WriteString("+" + clusterDash)
	for range waveColumns {
		b.WriteString("+" + waveDash)
	}
	b.WriteString("+" + compDash + "+\n")

	// Legend
	b.WriteString("  [=] completed  [ ] available  [x] locked\n")

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
	if filled > width {
		filled = width
	}
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

func displayWidth(s string) int {
	w := 0
	for _, r := range s {
		w += runeWidth(r)
	}
	return w
}

func padRight(s string, width int) string {
	dw := displayWidth(s)
	if dw >= width {
		return s
	}
	return s + strings.Repeat(" ", width-dw)
}

func center(s string, width int) string {
	dw := displayWidth(s)
	if dw >= width {
		return truncate(s, width)
	}
	pad := (width - dw) / 2
	return strings.Repeat(" ", pad) + s + strings.Repeat(" ", width-dw-pad)
}

func truncate(s string, maxWidth int) string {
	if displayWidth(s) <= maxWidth {
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
