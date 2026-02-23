package ui

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/mattn/go-runewidth"
)

const (
	minUIWidth  = 80
	minUIHeight = 24
)

var ansiEscapeRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

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
	line = sanitizeGlyphsPreserveANSI(line)
	if displayWidth(line) > width {
		line = trimToDisplayWidth(line, width)
	}
	w := displayWidth(line)
	if w == width {
		return line
	}
	return line + strings.Repeat(" ", width-w)
}

func centerLine(line string, width int) string {
	if width <= 0 {
		return line
	}
	line = sanitizeGlyphsPreserveANSI(line)
	w := displayWidth(line)
	if w >= width {
		return trimToDisplayWidth(line, width)
	}
	pad := (width - w) / 2
	return strings.Repeat(" ", pad) + line + strings.Repeat(" ", width-w-pad)
}

func stripANSI(s string) string {
	if s == "" {
		return s
	}
	return ansiEscapeRegex.ReplaceAllString(s, "")
}

func displayWidth(s string) int {
	return runewidth.StringWidth(stripANSI(s))
}

func trimToDisplayWidth(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if strings.Contains(s, "\x1b[") {
		s = stripANSI(s)
	}
	if displayWidth(s) <= width {
		return s
	}
	var b strings.Builder
	cur := 0
	for _, r := range s {
		r = normalizeGlyph(r)
		rw := runewidth.RuneWidth(r)
		if rw < 0 {
			rw = 0
		}
		if cur+rw > width {
			break
		}
		b.WriteRune(r)
		cur += rw
	}
	return b.String()
}

func strongThreat(threat string, instability string) string {
	t := strings.ToUpper(strings.TrimSpace(threat))
	i := strings.ToUpper(strings.TrimSpace(instability))
	if t == "" {
		t = "-"
	}
	if t == "CRITICAL" || i == "CRITICAL" {
		return styleDanger("CRITICAL", true)
	}
	if t == "HIGH" || i == "DANGEROUS" {
		return styleDanger("HIGH", false)
	}
	if t == "MEDIUM" || i == "UNSTABLE" {
		return styleWarn("MEDIUM")
	}
	return styleDim("LOW")
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

func channelBar(active bool, tick int, width int) string {
	if width <= 0 {
		width = 6
	}
	fill := 0
	if active {
		fill = clamp(tick, 1, width)
	}
	return "[" + strings.Repeat("=", fill) + strings.Repeat("-", width-fill) + "]"
}

func prioritizedEvent(m Model) string {
	if strings.TrimSpace(m.obs.Event) != "" {
		return sanitizeGlyphsPreserveANSI(m.obs.Event)
	}
	if m.obs.Dungeon != nil && strings.TrimSpace(m.obs.Dungeon.Event) != "" {
		return sanitizeGlyphsPreserveANSI(m.obs.Dungeon.Event)
	}
	if strings.TrimSpace(m.status) != "" {
		return sanitizeGlyphsPreserveANSI(m.status)
	}
	if currentPresentationOptions().ASCIIMode {
		return styleDim("...")
	}
	return styleDim("…")
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
	if height <= 0 {
		height = 24
	}

	if width < minUIWidth || height < minUIHeight {
		out := make([]string, 0, height)
		out = append(out, fitLine("Nightshade", width))
		for len(out) < height-1 {
			if len(out) == (height/2)-1 {
				out = append(out, fitLine("Terminal too small.", width))
			} else if len(out) == (height / 2) {
				out = append(out, fitLine("Resize to at least 80x24.", width))
			} else {
				out = append(out, strings.Repeat(" ", width))
			}
		}
		out = append(out, strings.Repeat(" ", width))
		return joinLines(out)
	}

	signalID := selectedSignalID(m.obs.Mode, lenSignalList(m.obs), cursorValue(m.obs), m.lastSignalID)
	header := fmt.Sprintf("MODE:%-7s  SIGNAL:%-6s  INSTABILITY:%-9s  ENERGY:%3d", strings.ToUpper(emptyDefault(m.obs.Mode, "board")), signalID, strings.ToUpper(emptyDefault(instability, "-")), m.energy)

	pressure, maxPressure := 0, 0
	coreIntegrity := 0
	threat := "-"
	exitBar := channelBar(false, 0, 6)
	if m.obs.Dungeon != nil {
		pressure = m.obs.Dungeon.Pressure
		maxPressure = m.obs.Dungeon.MaxPressure
		coreIntegrity = m.obs.Dungeon.CoreIntegrity
		threat = emptyDefault(m.obs.Dungeon.Threat, "-")
		exitBar = channelBar(m.obs.Dungeon.ExitChanneling, m.obs.Dungeon.ExitChannelTick, 6)
	}
	state := fmt.Sprintf("PRESSURE:%s %2d/%-2d  CORE:%3d%%  THREAT:%s  EXIT:%s", pressureBar(pressure, maxPressure, 12), pressure, maxPressure, coreIntegrity, strongThreat(threat, instability), exitBar)

	event := prioritizedEvent(m)
	footer := "WASD Move  E Observe  . Wait  F Ability  1/2/3 Path  Q Dive  R Resume  [ ] Replay  I Inspect  ? Help  Ctrl-C Quit"

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
				cells := make([]string, 0, len(row))
				for _, cell := range row {
					safe := normalizeGlyph(cell)
					switch cell {
					case '.':
						cells = append(cells, ". ")
					case '#':
						cells = append(cells, "##")
					case '@':
						cells = append(cells, "@@")
					case 'C', 'c':
						cells = append(cells, "CC")
					case 'E', 'e':
						cells = append(cells, "EE")
					case 'A', 'a':
						cells = append(cells, "AA")
					case '~':
						cells = append(cells, "~~")
					case 'm', 'h', 'x', 'v', '!':
						u := strings.ToUpper(string(safe))
						cells = append(cells, u+u)
					default:
						cells = append(cells, string(safe)+" ")
					}
				}
				viewport = append(viewport, strings.Join(cells, ""))
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
