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

// RenderNavigatorWithWaves renders the Link Navigator with actual wave data.
// Wave status symbols: [ ] available  [x] locked  [=] completed
func RenderNavigatorWithWaves(result *ScanResult, projectName string, waves []Wave, adrCount int, lastScanned *time.Time, strictnessLevel string, shibitoCount int) string {
	// Group waves by cluster name
	wavesByCluster := make(map[string][]Wave)
	for _, w := range waves {
		wavesByCluster[w.ClusterName] = append(wavesByCluster[w.ClusterName], w)
	}

	var b strings.Builder

	completePct := int(result.Completeness * 100)

	border := strings.Repeat("=", navigatorWidth)
	b.WriteString(fmt.Sprintf("+%s+\n", border))
	b.WriteString(fmt.Sprintf("|%s|\n", center("SIGHTJACK - Link Navigator", navigatorWidth)))
	projName := padRight(truncate(projectName, 20), 20)
	projRow := "  Project: " + projName + "  |  Completeness: " + fmt.Sprintf("%3d%%", completePct)
	b.WriteString("|" + padRight(projRow, navigatorWidth) + "|\n")
	adrRow := fmt.Sprintf("  ADRs: %d", adrCount)
	if strictnessLevel != "" {
		adrRow += fmt.Sprintf("  |  Strictness: %s", strictnessLevel)
	}
	b.WriteString("|" + padRight(adrRow, navigatorWidth) + "|\n")
	if shibitoCount > 0 {
		shibitoRow := fmt.Sprintf("  Shibito: %d", shibitoCount)
		b.WriteString("|" + padRight(shibitoRow, navigatorWidth) + "|\n")
	}
	if lastScanned != nil {
		sessionRow := fmt.Sprintf("  Session: resumed (last scan: %s)", lastScanned.Format("2006-01-02 15:04"))
		b.WriteString("|" + padRight(sessionRow, navigatorWidth) + "|\n")
	}
	b.WriteString(fmt.Sprintf("+%s+\n", border))

	b.WriteString(fmt.Sprintf("|%s|\n", strings.Repeat(" ", navigatorWidth)))
	header := fmt.Sprintf("  %-*s", maxClusterName, "Cluster")
	for i := 1; i <= waveColumns; i++ {
		header += fmt.Sprintf(" W%d  ", i)
	}
	header += "  Comp."
	b.WriteString(fmt.Sprintf("| %-*s|\n", navigatorWidth-1, header))

	separator := "  " + strings.Repeat("-", navigatorWidth-4)
	b.WriteString(fmt.Sprintf("| %-*s|\n", navigatorWidth-1, separator))

	for _, cluster := range result.Clusters {
		pct := int(cluster.Completeness * 100)
		name := padRight(truncate(cluster.Name, maxClusterName), maxClusterName)
		row := "  " + name

		clusterWaves := wavesByCluster[cluster.Name]
		for i := 0; i < waveColumns; i++ {
			if i < len(clusterWaves) {
				row += waveStatusSymbol(clusterWaves[i].Status)
			} else {
				row += "     " // empty cell (5 chars, matches waveStatusSymbol width)
			}
		}
		row += fmt.Sprintf("  %3d%%", pct)
		b.WriteString("|" + padRight(" "+row, navigatorWidth) + "|\n")
	}

	b.WriteString(fmt.Sprintf("|%s|\n", strings.Repeat(" ", navigatorWidth)))
	b.WriteString(fmt.Sprintf("+%s+\n", border))
	b.WriteString(fmt.Sprintf("| %-*s|\n", navigatorWidth-1, " [ ] available  [x] locked  [=] completed"))
	b.WriteString(fmt.Sprintf("+%s+\n", border))

	// Wave listing: show wave titles grouped by cluster
	for _, cluster := range result.Clusters {
		clusterWaves := wavesByCluster[cluster.Name]
		if len(clusterWaves) == 0 {
			continue
		}
		for i, w := range clusterWaves {
			line := fmt.Sprintf("  %s W%d: %s %s",
				cluster.Name, i+1, waveStatusSymbol(w.Status), w.Title)
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
