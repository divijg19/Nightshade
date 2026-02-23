package ui

import (
	"fmt"
	"strings"
)

func renderBoard(m Model) string {
	rows := []string{"IDX SIGNAL   TYPE       STATE    DECAY  CORR", "--- -------- ---------- -------- ----- -----"}
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
			line := fmt.Sprintf("%s%-3d %-8s %-10s %-8s %5d %5d", cursor, i+1, sig.ID, strings.ToUpper(sig.Type), state, sig.Decay, sig.Corruption)
			rows = append(rows, line)
		}
	} else {
		rows = append(rows, "NO SIGNAL DATA")
	}
	rows = append(rows, "")
	rows = append(rows, styleColor(ColorAccent, "QUICK DIVE: Q")+"   "+styleColor(ColorPrimary, "RESUME: R")+"   "+styleDim("ENTER: 1..9"))
	if m.obs.RunSummary != nil {
		rows = append(rows, "")
		rows = append(rows, fmt.Sprintf("RUN: %s", strings.ToUpper(emptyDefault(m.obs.RunSummary.ResultType, "UNKNOWN"))))
		rows = append(rows, fmt.Sprintf("PEAK: %d/%d  FRAG: +%d  XP: +%d", m.obs.RunSummary.PeakPressure, m.obs.RunSummary.MaxPressure, m.obs.RunSummary.FragmentsGained, m.obs.RunSummary.SkillXP))
	}
	event := prioritizedEvent(m)
	footer := styleColor(ColorPrimary, "MOVEMENT: ↑↓←→") + "   " + styleColor(ColorAccent, "ACTIONS: O H D") + "   " + styleDim("META: Q")
	if currentPresentationOptions().ASCIIMode {
		footer = styleColor(ColorPrimary, "MOVEMENT: WASD") + "   " + styleColor(ColorAccent, "ACTIONS: O H D") + "   " + styleDim("META: Q")
	}
	return layoutFramed(m, brandedHeader("SIGNAL BOARD"), "STABILITY: "+styleColor(ColorSuccess, "STABLE"), "PHASE: BOARD", ColorBorder, ColorPrimary, rows, event, footer)
}
