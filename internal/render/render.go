package render

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/core"
)

const (
	esc         = "\x1b"
	clearScreen = esc + "[2J"
	cursorHome  = esc + "[H"
	cursorHide  = esc + "[?25l"
	cursorShow  = esc + "[?25h"
)

const (
	ANSIReset        = "\x1b[0m"
	ANSIWhiteBright  = "\x1b[97m"
	ANSICyan         = "\x1b[36m"
	ANSIYellow       = "\x1b[33m"
	ANSIYellowBright = "\x1b[1;33m"
	ANSIYellowDim    = "\x1b[2;33m"
	ANSIRedBright    = "\x1b[1;31m"
	ANSIGrayDim      = "\x1b[90m"
	ANSIMagentaDim   = "\x1b[2;35m"
)

func color(code string, s string) string { return esc + "[" + code + "m" + s + esc + "[0m" }

func firstNonEmptyLine(s string) string {
	for _, ln := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(ln)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func parseEnergyDelta(status string) string {
	line := firstNonEmptyLine(status)
	if line == "" {
		return ""
	}
	start := strings.LastIndex(line, "(")
	end := strings.LastIndex(line, ")")
	if start < 0 || end <= start {
		return ""
	}
	inside := strings.TrimSpace(line[start+1 : end])
	if inside == "+0" || inside == "-0" || inside == "0" {
		return ""
	}
	if strings.HasPrefix(inside, "+") || strings.HasPrefix(inside, "-") {
		return inside
	}
	return ""
}

func threatColored(level string) string {
	up := strings.ToUpper(strings.TrimSpace(level))
	switch up {
	case "LOW":
		return ANSIGrayDim + "LOW" + ANSIReset
	case "MEDIUM":
		return ANSIYellow + "MEDIUM" + ANSIReset
	case "HIGH":
		return ANSIYellowBright + "HIGH" + ANSIReset
	case "EXTREME":
		return ANSIRedBright + "EXTREME" + ANSIReset
	default:
		return ANSIGrayDim + "LOW" + ANSIReset
	}
}

func oneEventLine(obsEvent string, status string) string {
	ev := firstNonEmptyLine(obsEvent)
	if ev != "" {
		if strings.Contains(ev, "You are entering a signal fragment") || strings.Contains(ev, "You are inside a signal") {
			return "You are stabilizing corrupted signals."
		}
		return ev
	}
	st := firstNonEmptyLine(status)
	if st == "" {
		return ""
	}
	if idx := strings.Index(st, "  Energy "); idx >= 0 {
		st = strings.TrimSpace(st[:idx])
	}
	return st
}

// Options controls purely-presentational layout decisions.
// It must not influence game state or determinism.
type Options struct {
	// Minimal reduces visual framing and vertical density.
	Minimal bool
	// Width overrides the computed frame width when > 0.
	Width int
}

// Frame represents a full UI frame assembled in memory.
type Frame struct {
	Tick      int
	Options   Options
	Header    string
	Grid      []string
	Narration []string
	Status    []string
	HUD       []string
	Prompt    string
}

// RenderGrid returns fixed-size, colorized grid lines for the observation.
// It does NOT perform any IO.
func RenderGrid(obs agent.Observation) []string {
	if obs.Mode == "board" && obs.Board != nil {
		if obs.RunSummary != nil {
			result := strings.Title(obs.RunSummary.ResultType)
			if result == "" {
				result = "Unknown"
			}
			return []string{
				"══════════════════════════════",
				"         SIGNAL REPORT",
				"══════════════════════════════",
				fmt.Sprintf("Result: %s", result),
				fmt.Sprintf("Pressure Peak: %d / %d", obs.RunSummary.PeakPressure, obs.RunSummary.MaxPressure),
				fmt.Sprintf("Fragments Gained: +%d", obs.RunSummary.FragmentsGained),
				fmt.Sprintf("Skill XP: +%d", obs.RunSummary.SkillXP),
				fmt.Sprintf("Time In Signal: %d ticks", obs.RunSummary.TimeInSignal),
				fmt.Sprintf("Threat Level: %s", obs.RunSummary.ThreatLevel),
				"",
				"[Press Enter to return]",
			}
		}
		lines := make([]string, 0, 8)
		maxCorruption := -1
		for _, s := range obs.Board.Signals {
			if s.Corruption > maxCorruption {
				maxCorruption = s.Corruption
			}
		}
		for i, s := range obs.Board.Signals {
			lead := "  "
			if i == obs.Board.Cursor {
				lead = "> "
			}
			icon := "○"
			if strings.EqualFold(s.Type, "FRACTURE") {
				icon = "✖"
			}
			if s.Burned || s.Decay <= 0 {
				icon = "✓"
			}
			label := "NODE"
			if strings.EqualFold(s.Anchor, "MEMORY_VAULT") {
				label = "VAULT"
			}
			marks := ""
			if s.Corruption == maxCorruption {
				marks += " *"
			}
			if obs.LastSignalID != "" && s.ID == obs.LastSignalID {
				marks += " ↺"
			}
			lines = append(lines, fmt.Sprintf("%s[%d] %s %-6s  ◆ %d%s", lead, i+1, icon, label, s.Corruption, marks))
			if i >= 4 {
				break
			}
		}
		lines = append(lines, "1–5 Enter   Q Quick Dive   R Resume")
		return lines
	}

	if obs.Mode == "dungeon" {
		lines := make([]string, 0, 8)
		if obs.Dungeon == nil {
			return []string{"(dungeon)"}
		}

		if len(obs.Dungeon.Grid) > 0 {
			mutatorMap := map[core.Position]agent.MutatorTileView{}
			for _, mt := range obs.Dungeon.MutatorTiles {
				mutatorMap[core.Position{X: mt.X, Y: mt.Y}] = mt
			}
			enemyMap := map[core.Position]agent.EnemyView{}
			for _, ev := range obs.Dungeon.Enemies {
				pos := core.Position{X: ev.X, Y: ev.Y}
				enemyMap[pos] = ev
			}
			for y, row := range obs.Dungeon.Grid {
				var b strings.Builder
				for x, ch := range row {
					pos := core.Position{X: x, Y: y}
					g := unicodeGlyph(ch)
					// center is player
					if x == int(obs.Position.X) && y == int(obs.Position.Y) {
						b.WriteString(ANSIWhiteBright + "@" + ANSIReset)
						continue
					}
					if ev, ok := enemyMap[pos]; ok {
						glyph := "H"
						switch ev.Kind {
						case "HUNTER":
							glyph = "H"
						case "SENTINEL":
							glyph = "S"
						case "WARDEN":
							glyph = "W"
						default:
							glyph = "H"
						}
						if ev.TargetLocked {
							b.WriteString(ANSIRedBright + glyph + ANSIReset)
						} else {
							b.WriteString(glyph)
						}
						continue
					}
					if mt, ok := mutatorMap[pos]; ok {
						switch mt.Type {
						case "CORRUPTION_ZONE":
							b.WriteString("x")
						case "STABILIZATION_FIELD":
							b.WriteString("+")
						case "FRAGILE_FLOOR":
							b.WriteString("o")
						case "ENEMY_NEST":
							b.WriteString("n")
						default:
							b.WriteRune(g)
						}
						continue
					}
					if obs.Dungeon.Phase == "II" && g == 'A' {
						b.WriteString(ANSIYellowBright + "◉" + ANSIReset)
						continue
					}
					if obs.Dungeon.Phase == "III" && g == '#' {
						b.WriteString(ANSIGrayDim + "#" + ANSIReset)
						continue
					}
					if obs.Dungeon.Phase == "III" && g == '·' && (obs.Tick%2 == 0) {
						b.WriteString(ANSIYellowDim + "·" + ANSIReset)
						continue
					}
					b.WriteRune(g)
				}
				lines = append(lines, b.String())
			}
		} else {
			lines = append(lines, "(no grid)")
		}
		return lines
	}

	// viewport radius (produces 5x5 grid)
	r := 2
	size := 2*r + 1

	// Prepare maps for quick lookup
	visMap := map[core.Position]core.TileView{}
	for _, v := range obs.Visible {
		visMap[v.Position] = v
	}
	knownMap := map[core.Position]agent.Belief{}
	for _, b := range obs.Known {
		knownMap[b.Tile.Position] = b
	}
	presenceMap := map[core.Position]agent.PresenceType{}
	for _, p := range obs.Presence {
		cur, ok := presenceMap[p.Position]
		if !ok {
			presenceMap[p.Position] = p.Type
			continue
		}
		prio := func(t agent.PresenceType) int {
			switch t {
			case agent.PresenceSelf:
				return 3
			case agent.PresenceHumanOther:
				return 2
			case agent.PresenceNPC:
				return 1
			default:
				return 0
			}
		}
		if prio(p.Type) > prio(cur) {
			presenceMap[p.Position] = p.Type
		}
	}

	center := obs.Position
	lines := make([]string, 0, size)
	for dy := -r; dy <= r; dy++ {
		var row strings.Builder
		for dx := -r; dx <= r; dx++ {
			pos := core.Position{X: center.X + dx, Y: center.Y + dy}
			// Default glyph
			var cell string
			// center = self
			if dx == 0 && dy == 0 {
				cell = ANSIWhiteBright + "@" + ANSIReset
			} else if pt, ok := presenceMap[pos]; ok {
				switch pt {
				case agent.PresenceSelf:
					cell = ANSIWhiteBright + "@" + ANSIReset
				case agent.PresenceHumanOther:
					cell = ANSICyan + "@" + ANSIReset
				case agent.PresenceNPC:
					cell = ANSIGrayDim + "@" + ANSIReset
				}
			} else if tv, ok := visMap[pos]; ok {
				// visible
				isHalluc := false
				if b, ok2 := knownMap[pos]; ok2 {
					if b.Age > agent.ParanoiaThreshold {
						isHalluc = true
					}
				}
				if isHalluc {
					cell = ANSIMagentaDim + string(tv.Glyph) + ANSIReset
				} else {
					cell = ANSIWhiteBright + string(tv.Glyph) + ANSIReset
				}
			} else if b, ok := knownMap[pos]; ok {
				cell = ANSIGrayDim + string(b.Tile.Glyph) + ANSIReset
			} else {
				cell = ANSIGrayDim + "?" + ANSIReset
			}
			row.WriteString(cell)
		}
		lines = append(lines, row.String())
	}
	return lines
}

// BuildFrame assembles the Frame in memory. No IO.

func splitStatusLines(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	// Keep status compact; avoid raw \n in output.
	parts := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	lines := make([]string, 0, 3)
	for _, p := range parts {
		p = strings.TrimRight(p, "\r\n")
		if p == "" {
			continue
		}
		lines = append(lines, p)
		if len(lines) == 3 {
			break
		}
	}
	return lines
}

func visibleWidth(ansiRE *regexp.Regexp, s string) int {
	vis := ansiRE.ReplaceAllString(s, "")
	return len([]rune(vis))
}

func maxVisibleWidth(ansiRE *regexp.Regexp, lines []string) int {
	max := 0
	for _, l := range lines {
		if w := visibleWidth(ansiRE, l); w > max {
			max = w
		}
	}
	return max
}

func BuildFrameWithOptions(obs agent.Observation, status string, energy int, paranoia int, scars int, opts Options) Frame {
	grid := RenderGrid(obs)
	narration := []string{}
	eventLine := oneEventLine(obs.Event, status)
	if eventLine != "" {
		narration = append(narration, eventLine)
	}

	// HUD
	hud := make([]string, 0, 2)
	if obs.Mode == "dungeon" {
		segments := 20
		filled := 0
		maxPressure := 20
		core := 100
		threat := "LOW"
		enraged := false
		phase := ""
		if obs.Dungeon != nil {
			filled = obs.Dungeon.Pressure
			maxPressure = obs.Dungeon.MaxPressure
			core = obs.Dungeon.CoreIntegrity
			threat = obs.Dungeon.Threat
			enraged = obs.Dungeon.Enraged
			phase = obs.Dungeon.Phase
		}
		if filled < 0 {
			filled = 0
		}
		if filled > segments {
			filled = segments
		}
		delta := parseEnergyDelta(status)
		energyPart := fmt.Sprintf("⚡ %d/100", energy)
		if delta != "" {
			energyPart += " (" + delta + ")"
		}
		state := fmt.Sprintf("◼ %d/%d   ◆ %d   ▲ %s   %s", filled, maxPressure, core, threatColored(threat), energyPart)
		statusLines := []string{state}
		bar := strings.Repeat("█", filled) + strings.Repeat("░", segments-filled)
		if obs.Dungeon != nil && (obs.Dungeon.InstabilityBand >= 3 || strings.EqualFold(threat, "EXTREME")) {
			bar = ANSIRedBright + bar + ANSIReset
		}
		if enraged && (obs.Tick%2 == 0) {
			bar = ANSIRedBright + bar + ANSIReset
		}
		statusLines = append(statusLines, fmt.Sprintf("◼ %d/%d  %s", filled, maxPressure, bar))
		if obs.Dungeon != nil && obs.Dungeon.ExitChanneling {
			fill := obs.Tick - obs.Dungeon.ExitChannelTick + 1
			if fill < 0 {
				fill = 0
			}
			if fill > 5 {
				fill = 5
			}
			statusLines = append(statusLines, fmt.Sprintf("⏳ %s%s", strings.Repeat("█", fill), strings.Repeat("░", 5-fill)))
		}
		_ = phase
		status = strings.Join(statusLines, "\n")
		hud = nil
		narration = []string{}
		if eventLine != "" {
			narration = append(narration, eventLine)
		}
	} else {
		hudLabel := color("2;36", "Energy")
		energyStr := fmt.Sprintf(" %d/%d", energy, 100)
		if energy < 30 {
			energyStr = color("1;33", energyStr)
		}
		hud = append(hud, fmt.Sprintf("%s%s", hudLabel, energyStr))
		hud = append(hud, fmt.Sprintf("P %d  S %d  Beliefs %d", paranoia, scars, len(obs.Known)))
	}

	statusLines := splitStatusLines(status)

	prompt := "> "
	if opts.Minimal {
		prompt = "> "
	}

	header := "WORLD  " + ANSIGrayDim + fmt.Sprintf("tick %d", obs.Tick) + ANSIReset
	if obs.Mode == "dungeon" {
		band := "STABLE"
		mut := ""
		if obs.Dungeon != nil {
			switch obs.Dungeon.InstabilityBand {
			case 1:
				band = "UNSTABLE"
			case 2:
				band = "DANGEROUS"
			case 3:
				band = "CRITICAL"
			}
			if len(obs.Dungeon.MutatorTiles) > 0 {
				mut = "  ✦ MUTATOR"
			}
		}
		enrage := ""
		if obs.Dungeon != nil && obs.Dungeon.Enraged {
			enrage = "  ‼"
		}
		header = fmt.Sprintf("DUNGEON  t=%d  %s%s%s", obs.Tick, band, enrage, mut)
	}
	if obs.Mode == "board" {
		header = "== SIGNAL BOARD =="
	}
	if obs.Mode == "board" && obs.RunSummary != nil {
		header = "SIGNAL REPORT"
	}

	return Frame{
		Tick:      obs.Tick,
		Options:   opts,
		Header:    header,
		Grid:      grid,
		Narration: narration,
		Status:    statusLines,
		HUD:       hud,
		Prompt:    prompt,
	}
}

func unicodeGlyph(ch rune) rune {
	switch ch {
	case '#', '%':
		return '#'
	case '.', '~':
		return '·'
	case 'A':
		return 'A'
	case 'X':
		return '◇'
	case 'E':
		return 'E'
	default:
		return ch
	}
}

// BuildFrame assembles the Frame in memory. No IO.
func BuildFrame(obs agent.Observation, ephemeral string, energy int, paranoia int, scars int) Frame {
	return BuildFrameWithOptions(obs, ephemeral, energy, paranoia, scars, Options{})
}

// RenderFrame performs the actual IO to the writer for a fully-built Frame.
func RenderFrame(w io.Writer, f Frame) {
	var sb strings.Builder
	writeLine := func(b *strings.Builder, s string) {
		b.WriteString(s)
		b.WriteString("\r\n")
	}

	// Absolute screen reset and home cursor
	sb.WriteString(clearScreen)
	sb.WriteString(cursorHome)
	sb.WriteString(cursorHide)

	ansiRE := regexp.MustCompile(`\x1b\[[0-9;]*m`)

	// Determine frame width (stable across frames for visual calm).
	// Use a fixed default unless explicitly overridden.
	const defaultWidth = 40
	frameW := defaultWidth
	if f.Options.Width > 0 {
		frameW = f.Options.Width
	}
	if frameW < 22 {
		frameW = 22
	}
	// Guardrail: if any line would exceed the configured width, expand
	// rather than truncating/misaligning (still capped below).
	header := f.Header
	if header == "" {
		header = "WORLD  " + ANSIGrayDim + fmt.Sprintf("tick %d", f.Tick) + ANSIReset
	}
	all := make([]string, 0, len(f.Grid)+len(f.Narration)+len(f.Status)+len(f.HUD)+1)
	all = append(all, header)
	all = append(all, f.Grid...)
	all = append(all, f.Narration...)
	all = append(all, f.Status...)
	all = append(all, f.HUD...)
	if need := maxVisibleWidth(ansiRE, all); need > frameW {
		frameW = need
	}
	if frameW > 60 {
		frameW = 60
	}
	sep := strings.Repeat("-", frameW)
	isDungeonFrame := strings.HasPrefix(header, "DUNGEON")
	if frameW > 80 {
		frameW = 80
		sep = strings.Repeat("-", frameW)
	}

	if isDungeonFrame {
		writeLine(&sb, header)
		writeLine(&sb, sep)
		for i, s := range f.Status {
			if i >= 3 {
				break
			}
			line := s
			if visibleWidth(ansiRE, line) > 80 {
				runes := []rune(ansiRE.ReplaceAllString(line, ""))
				if len(runes) > 80 {
					line = string(runes[:80])
				}
			}
			writeLine(&sb, line)
		}
		writeLine(&sb, sep)
		for _, line := range f.Grid {
			writeLine(&sb, line)
		}
		writeLine(&sb, sep)
		if len(f.Narration) > 0 {
			writeLine(&sb, f.Narration[0])
		}
		sb.WriteString(cursorShow)
		sb.WriteString(f.Prompt)
		_, _ = io.WriteString(w, sb.String())
		return
	}

	// Header + single separator
	writeLine(&sb, header)
	if !f.Options.Minimal && !isDungeonFrame {
		writeLine(&sb, sep)
	}

	// Grid lines (centered within frame width)
	for _, line := range f.Grid {
		visLen := visibleWidth(ansiRE, line)
		pad := 0
		if frameW > visLen {
			pad = (frameW - visLen) / 2
		}
		out := strings.Repeat(" ", pad) + line
		outVis := visibleWidth(ansiRE, out)
		if outVis < frameW {
			out += strings.Repeat(" ", frameW-outVis)
		}
		writeLine(&sb, out)
	}

	// Ambient narration (optional)
	for _, n := range f.Narration {
		if strings.TrimSpace(n) == "" {
			continue
		}
		writeLine(&sb, ANSIGrayDim+n+ANSIReset)
	}

	// Status (0..2 lines)
	for _, s := range f.Status {
		writeLine(&sb, s)
	}
	if len(f.Status) == 0 && len(f.Narration) == 0 && !isDungeonFrame {
		// keep one breathing line for stable prompt placement
		writeLine(&sb, "")
	}

	// HUD (compact)
	for _, h := range f.HUD {
		out := h
		outVis := visibleWidth(ansiRE, out)
		if outVis < frameW {
			out += strings.Repeat(" ", frameW-outVis)
		}
		writeLine(&sb, out)
	}
	if !f.Options.Minimal && !isDungeonFrame {
		writeLine(&sb, sep)
	}
	// Prompt line
	sb.WriteString(cursorShow)
	sb.WriteString(f.Prompt)

	// flush buffer to writer
	_, _ = io.WriteString(w, sb.String())
}

// RenderTo kept for external callers; builds frame and writes it.
func RenderTo(w io.Writer, obs agent.Observation, energy int, paranoia int, scars int, prompt string, ephemeral string) {
	f := BuildFrameWithOptions(obs, ephemeral, energy, paranoia, scars, Options{})
	// ensure prompt uses provided prompt string appended to default prompt
	if prompt != "" {
		f.Prompt = f.Prompt + prompt
	}
	RenderFrame(w, f)
}

// RenderToWithOptions is an optional entrypoint for callers that want minimal UI framing.
// It preserves determinism by remaining purely presentational.
func RenderToWithOptions(w io.Writer, obs agent.Observation, energy int, paranoia int, scars int, prompt string, status string, opts Options) {
	f := BuildFrameWithOptions(obs, status, energy, paranoia, scars, opts)
	if prompt != "" {
		f.Prompt = f.Prompt + prompt
	}
	RenderFrame(w, f)
}

// RenderForTest is a helper used by tests to render with default HUD values.
func RenderForTest(obs agent.Observation) string {
	var b strings.Builder
	RenderTo(&b, obs, 100, 3, 0, "", "")
	return b.String()
}
