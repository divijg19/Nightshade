package ui

import (
	"fmt"
	"regexp"
	"strings"

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
		return styleBoldColor(ColorCritical, "CRITICAL")
	}
	if t == "HIGH" || i == "DANGEROUS" {
		return styleColor(ColorDanger, "HIGH")
	}
	if t == "MEDIUM" || i == "UNSTABLE" {
		return styleColor(ColorWarning, "MEDIUM")
	}
	return styleColor(ColorSuccess, "LOW")
}

func pressureBar(pressure, maxPressure int, width int) string {
	if width <= 0 {
		width = 12
	}
	if maxPressure <= 0 {
		if currentPresentationOptions().ASCIIMode {
			return "[" + strings.Repeat("-", width) + "]"
		}
		return "[" + strings.Repeat("░", width) + "]"
	}
	ratio := float64(pressure) / float64(maxPressure)
	fill := int(ratio * float64(width))
	fill = clamp(fill, 0, width)

	var b strings.Builder
	b.WriteString("[")
	for i := 0; i < width; i++ {
		if i < fill {
			ch := "#"
			if !currentPresentationOptions().ASCIIMode {
				ch = "█"
			}
			r := float64(i+1) / float64(width)
			tok := ColorSuccess
			if r >= 0.80 {
				tok = ColorDanger
			} else if r >= 0.50 {
				tok = ColorWarning
			}
			b.WriteString(styleColor(tok, ch))
			continue
		}
		if currentPresentationOptions().ASCIIMode {
			b.WriteString("-")
		} else {
			b.WriteString(styleColor(ColorDim, "░"))
		}
	}
	b.WriteString("]")
	return b.String()
}

func channelBar(active bool, tick int, width int) string {
	if width <= 0 {
		width = 6
	}
	fill := 0
	if active {
		fill = clamp(tick, 1, width)
	}
	var b strings.Builder
	b.WriteString("[")
	for i := 0; i < width; i++ {
		if i < fill {
			b.WriteString(styleColor(ColorAccent, "="))
			continue
		}
		b.WriteString(styleColor(ColorDim, "-"))
	}
	b.WriteString("]")
	return b.String()
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

type frameCharset struct {
	tl rune
	tr rune
	bl rune
	br rune
	h  rune
	v  rune
	lm rune
	rm rune
}

func currentFrameCharset() frameCharset {
	if currentPresentationOptions().ASCIIMode {
		return frameCharset{tl: '+', tr: '+', bl: '+', br: '+', h: '-', v: '|', lm: '+', rm: '+'}
	}
	return frameCharset{tl: '╭', tr: '╮', bl: '╰', br: '╯', h: '─', v: '│', lm: '├', rm: '┤'}
}

func frameLine(left rune, fill rune, right rune, width int, token SemanticColor) string {
	if width <= 1 {
		return ""
	}
	middle := strings.Repeat(string(fill), width-2)
	return styleColor(token, string(left)+middle+string(right))
}

func frameContent(width int, text string, borderToken SemanticColor) string {
	if width <= 1 {
		return ""
	}
	chars := currentFrameCharset()
	inner := fitLine(text, width-2)
	return styleColor(borderToken, string(chars.v)) + inner + styleColor(borderToken, string(chars.v))
}

func stabilityToken(stability string) SemanticColor {
	s := strings.ToUpper(strings.TrimSpace(stability))
	switch s {
	case "CRITICAL":
		return ColorCritical
	case "DANGEROUS", "HIGH":
		return ColorDanger
	case "UNSTABLE", "MEDIUM":
		return ColorWarning
	default:
		return ColorSuccess
	}
}

func phaseToken(phase string) SemanticColor {
	p := strings.ToUpper(strings.TrimSpace(phase))
	if p == "" || p == "-" {
		return ColorMuted
	}
	if p == "HUNTER" || p == "AGGRESSOR" {
		return ColorHighlight
	}
	if p == "STABILIZER" {
		return ColorSuccess
	}
	if p == "HARVESTER" {
		return ColorWarning
	}
	return ColorAccent
}

func brandedHeader(suffix string) string {
	if currentPresentationOptions().ASCIIMode {
		return "NIGHTSHADE - " + strings.ToUpper(suffix)
	}
	return "NIGHTSHADE — " + strings.ToUpper(suffix)
}

func layoutFramed(m Model, title string, subtitleLeft string, subtitleRight string, borderToken SemanticColor, primaryToken SemanticColor, bodyLines []string, eventLine string, footerLine string) string {
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

	chars := currentFrameCharset()
	innerWidth := width - 2
	if innerWidth < 1 {
		innerWidth = 1
	}

	titleLine := styleColor(primaryToken, strings.ToUpper(title))
	subLine := subtitleLeft
	if strings.TrimSpace(subtitleRight) != "" {
		rightLabel := styleColor(phaseToken(subtitleRight), strings.ToUpper(subtitleRight))
		rightWidth := displayWidth(rightLabel)
		left := trimToDisplayWidth(subLine, innerWidth)
		leftWidth := displayWidth(left)
		if rightWidth < innerWidth && leftWidth+1+rightWidth <= innerWidth {
			left = trimToDisplayWidth(left, innerWidth-rightWidth-1)
			leftWidth = displayWidth(left)
			left = left + strings.Repeat(" ", innerWidth-leftWidth-rightWidth) + rightLabel
		}
		subLine = left
	}

	fixedRows := 9
	bodyRows := height - fixedRows
	if bodyRows < 1 {
		bodyRows = 1
	}
	start := 0
	if len(bodyLines) > bodyRows {
		start = (len(bodyLines) - bodyRows) / 2
	}
	end := start + bodyRows
	if end > len(bodyLines) {
		end = len(bodyLines)
	}
	trimmed := bodyLines[start:end]
	padding := bodyRows - len(trimmed)

	lines := make([]string, 0, height)
	lines = append(lines, frameLine(chars.tl, chars.h, chars.tr, width, borderToken))
	lines = append(lines, frameContent(width, fitLine(titleLine, innerWidth), borderToken))
	lines = append(lines, frameContent(width, fitLine(subLine, innerWidth), borderToken))
	lines = append(lines, frameLine(chars.lm, chars.h, chars.rm, width, borderToken))
	for i := 0; i < padding/2; i++ {
		lines = append(lines, frameContent(width, strings.Repeat(" ", innerWidth), borderToken))
	}
	for _, row := range trimmed {
		lines = append(lines, frameContent(width, fitLine(centerLine(row, innerWidth), innerWidth), borderToken))
	}
	for len(lines) < 4+bodyRows {
		lines = append(lines, frameContent(width, strings.Repeat(" ", innerWidth), borderToken))
	}
	lines = append(lines, frameLine(chars.lm, chars.h, chars.rm, width, borderToken))
	lines = append(lines, frameContent(width, fitLine(eventLine, innerWidth), borderToken))
	lines = append(lines, frameLine(chars.lm, chars.h, chars.rm, width, borderToken))
	lines = append(lines, frameContent(width, fitLine(footerLine, innerWidth), borderToken))
	lines = append(lines, frameLine(chars.bl, chars.h, chars.br, width, borderToken))
	if len(lines) > height {
		lines = lines[:height]
	}
	return joinLines(lines)
}

func layout5Zones(m Model, viewport []string, instability string) string {
	phase := "-"
	pressure, maxPressure := 0, 0
	coreIntegrity := 0
	threat := "-"
	exitBar := channelBar(false, 0, 6)
	enraged := false
	if m.obs.Dungeon != nil {
		pressure = m.obs.Dungeon.Pressure
		maxPressure = m.obs.Dungeon.MaxPressure
		coreIntegrity = m.obs.Dungeon.CoreIntegrity
		threat = emptyDefault(m.obs.Dungeon.Threat, "-")
		exitBar = channelBar(m.obs.Dungeon.ExitChanneling, m.obs.Dungeon.ExitChannelTick, 6)
		phase = emptyDefault(m.obs.Dungeon.Phase, "-")
		enraged = m.obs.Dungeon.Enraged
	}
	state1 := fmt.Sprintf("PRESSURE: %s  %2d/%-2d", pressureBar(pressure, maxPressure, 12), pressure, maxPressure)
	state2 := fmt.Sprintf("CORE: %3d%%  THREAT: %s  CHANNEL: %s", coreIntegrity, strongThreat(threat, instability), exitBar)

	event := prioritizedEvent(m)
	footer := styleColor(ColorPrimary, "MOVEMENT ↑↓←→") + "   " + styleColor(ColorAccent, "ACTIONS O H D") + "   " + styleDim("Q Quit")
	if currentPresentationOptions().ASCIIMode {
		footer = styleColor(ColorPrimary, "MOVEMENT WASD") + "   " + styleColor(ColorAccent, "ACTIONS O H D") + "   " + styleDim("Q Quit")
	}

	borderToken := ColorBorder
	if strings.EqualFold(instability, "CRITICAL") {
		borderToken = ColorCritical
	}
	headerToken := ColorPrimary
	if enraged {
		headerToken = ColorHighlight
	}

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

	chars := currentFrameCharset()
	innerWidth := width - 2
	if innerWidth < 1 {
		innerWidth = 1
	}

	stabilityLine := "STABILITY: " + styleColor(stabilityToken(instability), strings.ToUpper(emptyDefault(instability, "STABLE")))
	phaseLine := "PHASE: " + styleColor(phaseToken(phase), strings.ToUpper(phase))

	fixedRows := 13
	gridRows := height - fixedRows
	if gridRows < 1 {
		gridRows = 1
	}
	start := 0
	if len(viewport) > gridRows {
		start = (len(viewport) - gridRows) / 2
	}
	end := start + gridRows
	if end > len(viewport) {
		end = len(viewport)
	}
	trimmed := viewport[start:end]
	gridPad := gridRows - len(trimmed)

	lines := make([]string, 0, height)
	lines = append(lines, frameLine(chars.tl, chars.h, chars.tr, width, borderToken))
	lines = append(lines, frameContent(width, fitLine(styleColor(headerToken, brandedHeader("DUNGEON")), innerWidth), borderToken))
	lines = append(lines, frameContent(width, fitLine(stabilityLine, innerWidth), borderToken))
	lines = append(lines, frameContent(width, fitLine(phaseLine, innerWidth), borderToken))
	lines = append(lines, frameLine(chars.lm, chars.h, chars.rm, width, borderToken))
	lines = append(lines, frameContent(width, fitLine(state1, innerWidth), borderToken))
	lines = append(lines, frameContent(width, fitLine(state2, innerWidth), borderToken))
	lines = append(lines, frameLine(chars.lm, chars.h, chars.rm, width, borderToken))
	for i := 0; i < gridPad/2; i++ {
		lines = append(lines, frameContent(width, strings.Repeat(" ", innerWidth), borderToken))
	}
	for _, row := range trimmed {
		lines = append(lines, frameContent(width, fitLine(centerLine(row, innerWidth), innerWidth), borderToken))
	}
	for len(lines) < 8+gridRows {
		lines = append(lines, frameContent(width, strings.Repeat(" ", innerWidth), borderToken))
	}
	lines = append(lines, frameLine(chars.lm, chars.h, chars.rm, width, borderToken))
	lines = append(lines, frameContent(width, fitLine(event, innerWidth), borderToken))
	lines = append(lines, frameLine(chars.lm, chars.h, chars.rm, width, borderToken))
	lines = append(lines, frameContent(width, fitLine(footer, innerWidth), borderToken))
	lines = append(lines, frameLine(chars.bl, chars.h, chars.br, width, borderToken))
	if len(lines) > height {
		lines = lines[:height]
	}
	return joinLines(lines)
}

func renderSplash(m Model) string {
	width := m.width
	if width <= 0 {
		width = 80
	}
	height := m.height
	if height <= 0 {
		height = 24
	}
	if width < minUIWidth || height < minUIHeight {
		return fitLine("Terminal too small. Resize to at least 80x24.", width)
	}
	borderToken := ColorBorder
	inner := width - 2
	lines := []string{
		"",
		centerLine(styleBoldColor(ColorPrimary, "NIGHTSHADE"), inner),
		centerLine(styleColor(ColorAccent, "v0.3.18"), inner),
		"",
		centerLine(styleColor(ColorMuted, "A deterministic signal"), inner),
		centerLine(styleColor(ColorMuted, "stabilization protocol"), inner),
		"",
		centerLine(styleColor(ColorHighlight, "Press any key"), inner),
		"",
	}
	return layoutFramed(m, "", "", "", borderToken, ColorPrimary, lines, "", "")
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
						cells = append(cells, styleColor(ColorHighlight, "@")+" ")
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
