package ui

import (
	"fmt"
	"strings"
)

func renderBoard(m Model) string {
	viewport := []string{"IDX  SIGNAL  TYPE       STATE    DECAY  CORR", "------------------------------------------"}
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
			line := fmt.Sprintf("%s%-3d %-7s %-10s %-8s %5d %5d", cursor, i+1, sig.ID, strings.ToUpper(sig.Type), state, sig.Decay, sig.Corruption)
			viewport = append(viewport, line)
		}
		viewport = append(viewport, "\x1b[2mQ quick-dive  R resume-last  1..9 enter-signal\x1b[0m")
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
