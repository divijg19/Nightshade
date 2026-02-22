package ui

import "fmt"

func renderSummary(m Model) string {
	viewport := []string{"RUN SUMMARY", ""}
	if m.obs.RunSummary != nil {
		rs := m.obs.RunSummary
		viewport = append(viewport, fmt.Sprintf("Result: %s", emptyDefault(rs.ResultType, "unknown")))
		viewport = append(viewport, fmt.Sprintf("Peak Pressure: %d/%d", rs.PeakPressure, rs.MaxPressure))
		viewport = append(viewport, fmt.Sprintf("Fragments: +%d", rs.FragmentsGained))
		viewport = append(viewport, fmt.Sprintf("Skill XP: +%d", rs.SkillXP))
		viewport = append(viewport, fmt.Sprintf("Time in Signal: %d", rs.TimeInSignal))
		viewport = append(viewport, fmt.Sprintf("Threat: %s", emptyDefault(rs.ThreatLevel, "-")))
	} else {
		viewport = append(viewport, "No run summary available.")
	}
	return layout5Zones(m, viewport, "STABLE")
}
