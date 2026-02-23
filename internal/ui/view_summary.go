package ui

import "fmt"

func renderSummary(m Model) string {
	rows := []string{"RUN SUMMARY", ""}
	if m.obs.RunSummary != nil {
		rs := m.obs.RunSummary
		rows = append(rows, fmt.Sprintf("RESULT: %s", emptyDefault(rs.ResultType, "unknown")))
		rows = append(rows, fmt.Sprintf("PEAK PRESSURE: %d/%d", rs.PeakPressure, rs.MaxPressure))
		rows = append(rows, fmt.Sprintf("FRAGMENTS: +%d", rs.FragmentsGained))
		rows = append(rows, fmt.Sprintf("SKILL XP: +%d", rs.SkillXP))
		rows = append(rows, fmt.Sprintf("TIME IN SIGNAL: %d", rs.TimeInSignal))
		rows = append(rows, fmt.Sprintf("THREAT: %s", emptyDefault(rs.ThreatLevel, "-")))
	} else {
		rows = append(rows, "NO RUN SUMMARY AVAILABLE")
	}
	return layoutFramed(m, brandedHeader("SIGNAL BOARD"), "STABILITY: "+styleColor(ColorSuccess, "STABLE"), "PHASE: SUMMARY", ColorBorder, ColorPrimary, rows, prioritizedEvent(m), styleDim("META: Q"))
}
