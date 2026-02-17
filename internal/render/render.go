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
		lines := make([]string, 0, 8)
		threat := "LOW"
		if obs.Dungeon != nil && obs.Dungeon.Threat != "" {
			threat = obs.Dungeon.Threat
		}
		lines = append(lines, fmt.Sprintf("WORLD Epoch %d  Threat %s", obs.Tick/100, threat))
		lines = append(lines, "----------------------------------------")
		for i, s := range obs.Board.Signals {
			active := ""
			if i == obs.Board.Cursor {
				active = " [ACTIVE]"
			}
			stability := s.Decay
			corruption := 12 - s.Decay
			if corruption < 0 {
				corruption = 0
			}
			lines = append(lines, fmt.Sprintf("[%d] %s  S:%d  C:%d%s", i+1, s.Type, stability, corruption, active))
			if i >= 4 {
				break
			}
		}
		lines = append(lines, "----------------------------------------")
		if strings.TrimSpace(obs.Event) == "" {
			lines = append(lines, "Press 1–5 to enter  |  Q Quick Dive  |  R Resume Last")
		}
		return lines
	}

	if obs.Mode == "dungeon" {
		lines := make([]string, 0, 14)
		if obs.Dungeon == nil {
			return []string{"(dungeon)"}
		}

		pressureFilled := obs.Dungeon.Pressure
		if pressureFilled < 0 {
			pressureFilled = 0
		}
		if pressureFilled > obs.Dungeon.MaxPressure {
			pressureFilled = obs.Dungeon.MaxPressure
		}
		pressureEmpty := obs.Dungeon.MaxPressure - pressureFilled
		if pressureEmpty < 0 {
			pressureEmpty = 0
		}
		pressureBar := strings.Repeat("|", pressureFilled) + strings.Repeat("-", pressureEmpty)
		pressureLabel := fmt.Sprintf("PRESSURE %d/%d", obs.Dungeon.Pressure, obs.Dungeon.MaxPressure)
		if obs.Dungeon.InstabilityBand >= 3 || obs.Dungeon.Enraged {
			pressureLabel = ANSIRedBright + pressureLabel + ANSIReset
			pressureBar = ANSIRedBright + pressureBar + ANSIReset
		}
		threat := obs.Dungeon.Threat
		if threat == "" {
			threat = "LOW"
		}
		lines = append(lines, fmt.Sprintf("%s  %s    CORE %d%%    THREAT %s", pressureLabel, pressureBar, obs.Dungeon.CoreIntegrity, threat))

		eventLine := ""
		if strings.TrimSpace(obs.Event) != "" {
			eventLine = strings.TrimSpace(obs.Event)
		} else if strings.TrimSpace(obs.Dungeon.Event) != "" {
			eventLine = strings.TrimSpace(obs.Dungeon.Event)
		} else if obs.Dungeon.ExitChanneling {
			fill := obs.Tick - obs.Dungeon.ExitChannelTick + 1
			if fill < 0 {
				fill = 0
			}
			if fill > 5 {
				fill = 5
			}
			eventLine = fmt.Sprintf("EXIT CHANNEL: %s%s", strings.Repeat("█", fill), strings.Repeat("░", 5-fill))
		}
		if eventLine != "" {
			if strings.Contains(eventLine, "\n") {
				for _, ln := range strings.Split(eventLine, "\n") {
					ln = strings.TrimSpace(ln)
					if ln != "" {
						lines = append(lines, ln)
					}
				}
			} else {
				lines = append(lines, eventLine)
			}
		}

		if len(obs.Dungeon.Grid) > 0 {
			band := obs.Dungeon.InstabilityBand
			// build enemy lookup map for overlay
			enemyMap := map[core.Position]agent.EnemyView{}
			distractedAnchor := false
			distractedExit := false
			if obs.Dungeon != nil {
				for _, ev := range obs.Dungeon.Enemies {
					pos := core.Position{X: ev.X, Y: ev.Y}
					enemyMap[pos] = ev
					if ev.Target == "anchor" {
						distractedAnchor = true
					}
					if ev.Target == "exit" {
						distractedExit = true
					}
				}
			}
			for y, row := range obs.Dungeon.Grid {
				var b strings.Builder
				for x, ch := range row {
					pos := core.Position{X: x, Y: y}
					g := applyInstabilityGlyph(ch, band, x, y, obs.Tick)
					// center is player
					if x == int(obs.Position.X) && y == int(obs.Position.Y) {
						b.WriteString(ANSIWhiteBright + "@" + ANSIReset)
						continue
					}
					if _, ok := enemyMap[pos]; ok {
						// render archetype glyphs/colors
						ev := enemyMap[pos]
						switch ev.Kind {
						case "HUNTER":
							b.WriteString(ANSIRedBright + "H" + ANSIReset)
						case "SENTINEL":
							b.WriteString(ANSIYellow + "S" + ANSIReset)
						case "WARDEN":
							b.WriteString(ANSIRedBright + "W" + ANSIReset)
						case "SHADE":
							b.WriteString(ANSIMagentaDim + "D" + ANSIReset)
						default:
							b.WriteString(ANSIRedBright + "H" + ANSIReset)
						}
						continue
					}
					// v0.3.5: subtle cue when an enemy is distracted toward anchor/exit.
					if distractedAnchor && g == 'A' {
						b.WriteString(ANSIMagentaDim + "A" + ANSIReset)
						continue
					}
					if distractedExit && g == 'X' {
						b.WriteString(ANSIMagentaDim + "X" + ANSIReset)
						continue
					}
					if g == 'X' && (obs.Dungeon.InstabilityBand >= 3 || obs.Dungeon.Enraged) {
						b.WriteString(ANSIRedBright + "X" + ANSIReset)
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
	// Keep status to at most 2 lines; avoid raw \n in output.
	parts := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	lines := make([]string, 0, 2)
	for _, p := range parts {
		p = strings.TrimRight(p, "\r\n")
		if p == "" {
			continue
		}
		lines = append(lines, p)
		if len(lines) == 2 {
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
	// narration: v0.3.10 one-line rule
	narration := make([]string, 0, 1)
	if obs.Mode != "dungeon" {
		if obs.Event != "" {
			narration = append(narration, obs.Event)
		} else if !opts.Minimal && len(obs.Presence) > 0 {
			narration = append(narration, "You sense presences nearby.")
		}
	}

	// HUD (compact). Dungeon mode keeps top hierarchy minimal.
	hud := make([]string, 0, 2)
	if obs.Mode != "dungeon" {
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
		band := 0
		if obs.Dungeon != nil {
			band = obs.Dungeon.InstabilityBand
		}
		_, colored := instabilityLabel(band)
		enrage := ""
		if obs.Dungeon != nil && obs.Dungeon.Enraged {
			enrage = "  " + ANSIRedBright + "ENRAGE 1" + ANSIReset
		}
		header = fmt.Sprintf("DUNGEON  tick %d   [%s]%s", obs.Tick, colored, enrage)
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

func instabilityLabel(band int) (plain string, colored string) {
	switch band {
	case 0:
		plain = "STABLE"
		return plain, plain
	case 1:
		plain = "UNSTABLE"
		return plain, ANSIYellowDim + plain + ANSIReset
	case 2:
		plain = "DANGEROUS"
		return plain, ANSIYellowBright + plain + ANSIReset
	default:
		plain = "CRITICAL"
		return plain, ANSIRedBright + plain + ANSIReset
	}
}

func applyInstabilityGlyph(ch rune, band int, x int, y int, tick int) rune {
	// Never alter special markers.
	if ch == 'E' || ch == 'A' || ch == 'X' {
		return ch
	}

	out := ch

	// Band >=1: some floors wobble '.' -> '~'
	if band >= 1 && out == '.' {
		if (x+y+tick)%4 == 0 {
			out = '~'
		}
	}

	// Band >=2: some walls flicker '#' <-> '%'
	if band >= 2 && out == '#' {
		if (x+y+tick)%3 == 0 {
			out = '%'
		}
	}

	// Band >=3: unknown tiles appear more frequently (visual only)
	if band >= 3 {
		if out == '.' || out == '~' {
			if (x+y+tick)%4 == 1 {
				out = '?'
			}
		}
	}

	return out
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

	// Header + single separator
	writeLine(&sb, header)
	if !f.Options.Minimal {
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
	if len(f.Status) == 0 && len(f.Narration) == 0 {
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
	if !f.Options.Minimal {
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
