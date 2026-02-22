package ui

import (
	"fmt"
	"strings"

	"github.com/divijg19/Nightshade/internal/agent"
)

func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n")
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func fitLine(line string, width int) string {
	if width <= 0 {
		return line
	}
	runes := []rune(line)
	if len(runes) > width {
		return string(runes[:width])
	}
	if len(runes) == width {
		return line
	}
	return line + strings.Repeat(" ", width-len(runes))
}

func centerLine(line string, width int) string {
	if width <= 0 {
		return line
	}
	r := []rune(line)
	if len(r) >= width {
		return string(r[:width])
	}
	pad := (width - len(r)) / 2
	return strings.Repeat(" ", pad) + line + strings.Repeat(" ", width-len(r)-pad)
}

func pressureBar(pressure, maxPressure int, width int) string {
	if width <= 0 {
		width = 12
	}
	if maxPressure <= 0 {
		return "[" + strings.Repeat("-", width) + "]"
	}
	ratio := float64(pressure) / float64(maxPressure)
	fill := int(ratio * float64(width))
	fill = clamp(fill, 0, width)
	return "[" + strings.Repeat("#", fill) + strings.Repeat("-", width-fill) + "]"
}

func prioritizedEvent(m Model) string {
	if strings.TrimSpace(m.obs.Event) != "" {
		return m.obs.Event
	}
	if m.obs.Dungeon != nil && strings.TrimSpace(m.obs.Dungeon.Event) != "" {
		return m.obs.Dungeon.Event
	}
	if strings.TrimSpace(m.status) != "" {
		return m.status
	}
	return ""
}

func selectedSignalID(obsMode string, boardSignals int, boardCursor int, signalID string) string {
	if signalID != "" {
		return signalID
	}
	if obsMode != "board" || boardSignals == 0 {
		return "-"
	}
	idx := clamp(boardCursor, 0, boardSignals-1)
	return fmt.Sprintf("#%d", idx+1)
}

func layout5Zones(m Model, viewport []string, instability string) string {
	width := m.width
	if width <= 0 {
		width = 80
	}
	height := m.height

	signalID := selectedSignalID(m.obs.Mode, lenSignalList(m.obs), cursorValue(m.obs), m.lastSignalID)
	header := fmt.Sprintf("MODE:%s  SIGNAL:%s  INSTABILITY:%s", strings.ToUpper(emptyDefault(m.obs.Mode, "board")), signalID, emptyDefault(instability, "-"))

	pressure, maxPressure := 0, 0
	coreIntegrity := 0
	threat := "-"
	if m.obs.Dungeon != nil {
		pressure = m.obs.Dungeon.Pressure
		maxPressure = m.obs.Dungeon.MaxPressure
		coreIntegrity = m.obs.Dungeon.CoreIntegrity
		threat = emptyDefault(m.obs.Dungeon.Threat, "-")
	}
	state := fmt.Sprintf("P:%s %d/%d  CORE:%d%%  THREAT:%s", pressureBar(pressure, maxPressure, 12), pressure, maxPressure, coreIntegrity, strings.ToUpper(threat))

	event := prioritizedEvent(m)
	footer := "w/a/s/d move  e observe  . wait  f ability  1/2/3 path  q dive  r resume  [ ] replay  i inspect  ? help  Ctrl-C quit"

	if height <= 0 {
		lines := make([]string, 0, 4+len(viewport)+1)
		lines = append(lines, fitLine(header, width))
		lines = append(lines, fitLine(state, width))
		for _, row := range viewport {
			lines = append(lines, fitLine(centerLine(row, width), width))
		}
		lines = append(lines, fitLine(event, width))
		lines = append(lines, fitLine(footer, width))
		return joinLines(lines)
	}

	viewportRows := height - 4
	if viewportRows < 1 {
		viewportRows = 1
	}

	start := 0
	if len(viewport) > viewportRows {
		start = (len(viewport) - viewportRows) / 2
	}
	end := start + viewportRows
	if end > len(viewport) {
		end = len(viewport)
	}
	trimmed := viewport[start:end]

	gridLines := make([]string, 0, viewportRows)
	verticalPad := viewportRows - len(trimmed)
	for i := 0; i < verticalPad/2; i++ {
		gridLines = append(gridLines, strings.Repeat(" ", width))
	}
	for _, row := range trimmed {
		gridLines = append(gridLines, fitLine(centerLine(row, width), width))
	}
	for len(gridLines) < viewportRows {
		gridLines = append(gridLines, strings.Repeat(" ", width))
	}

	lines := make([]string, 0, height)
	lines = append(lines, fitLine(header, width))
	lines = append(lines, fitLine(state, width))
	lines = append(lines, gridLines...)
	lines = append(lines, fitLine(event, width))
	lines = append(lines, fitLine(footer, width))
	if len(lines) > height {
		lines = lines[:height]
	}
	return joinLines(lines)
}

func renderDungeon(m Model) string {
	viewport := []string{"(no dungeon view)"}
	instability := "-"
	if m.obs.Dungeon != nil {
		if len(m.obs.Dungeon.Grid) > 0 {
			viewport = make([]string, 0, len(m.obs.Dungeon.Grid))
			for _, row := range m.obs.Dungeon.Grid {
				viewport = append(viewport, string(row))
			}
		}
		instability = emptyDefault(m.obs.Dungeon.InstabilityLabel, "-")
	}
	return layout5Zones(m, viewport, instability)
}

func emptyDefault(s, d string) string {
	if strings.TrimSpace(s) == "" {
		return d
	}
	return s
}

func lenSignalList(obs agent.Observation) int {
	if obs.Board == nil {
		return 0
	}
	return len(obs.Board.Signals)
}

func cursorValue(obs agent.Observation) int {
	if obs.Board == nil {
		return 0
	}
	return obs.Board.Cursor
}
