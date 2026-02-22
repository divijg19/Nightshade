package ui

import (
	"fmt"
	"strings"
)

func boardRow(i int, s string) string {
	if len(s) > 72 {
		s = s[:72]
	}
	return fmt.Sprintf("%d) %s", i+1, s)
}

func renderBoard(m Model) string {
	viewport := []string{"SIGNAL BOARD", ""}
	instability := "STABLE"
	if m.obs.Board != nil {
		for i, sig := range m.obs.Board.Signals {
			state := "OPEN"
			if sig.Burned {
				state = "BURNED"
			} else if sig.Locked {
				state = "LOCKED"
			}
			cursor := " "
			if i == m.obs.Board.Cursor {
				cursor = ">"
			}
			line := fmt.Sprintf("%s%s %s [%s] D:%d C:%d", cursor, sig.ID, strings.ToUpper(sig.Type), state, sig.Decay, sig.Corruption)
			viewport = append(viewport, boardRow(i, line))
		}
	} else {
		viewport = append(viewport, "No signal data.")
	}
	if m.obs.RunSummary != nil {
		viewport = append(viewport, "")
		viewport = append(viewport, fmt.Sprintf("RUN %s", strings.ToUpper(emptyDefault(m.obs.RunSummary.ResultType, "unknown"))))
		viewport = append(viewport, fmt.Sprintf("PEAK %d/%d  FRAG +%d  XP +%d", m.obs.RunSummary.PeakPressure, m.obs.RunSummary.MaxPressure, m.obs.RunSummary.FragmentsGained, m.obs.RunSummary.SkillXP))
	}
	return layout5Zones(m, viewport, instability)
}
