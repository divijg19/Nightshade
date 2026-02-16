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
		// compact board UI (v0.3.9)
		lines := make([]string, 0, 6)
		// compact header
		lines = append(lines, fmt.Sprintf("WORLD Epoch %d  Threat %s", obs.Tick/100, obs.Dungeon.Threat))
		lines = append(lines, "----------------------------------------")
		// compact signals list (1..5)
		for i, s := range obs.Board.Signals {
			active := ""
			if i == obs.Board.Cursor {
				active = "ACTIVE"
			}
			lines = append(lines, fmt.Sprintf("[%d] %s  S:%d  C:%d %s", i+1, s.Type, s.Decay, s.Decay/2, active))
			if i >= 4 {
				break
			}
		}
		lines = append(lines, "----------------------------------------")
		lines = append(lines, "Press 1–5 to enter | Q Quick Dive")
		return lines
	}

	if obs.Mode == "dungeon" {
		lines := make([]string, 0, 16)
		if obs.Dungeon == nil {
			return []string{"(dungeon)"}
		}
		// pressure HUD redesign (v0.3.9)
		// Build label
		if obs.Dungeon.BuildLabel != "" {
			lines = append(lines, fmt.Sprintf("BUILD: %s", obs.Dungeon.BuildLabel))
		}
		// Pressure line with band label
		bandLabel := obs.Dungeon.InstabilityLabel
		if bandLabel == "" {
			bandLabel = "STABLE"
		}
		lines = append(lines, fmt.Sprintf("PRESSURE %d/%d  [%s]", obs.Dungeon.Pressure, obs.Dungeon.MaxPressure, bandLabel))
		// bar
		filled := obs.Dungeon.Pressure
		empty := obs.Dungeon.MaxPressure - obs.Dungeon.Pressure
		if filled < 0 {
			filled = 0
		}
		if empty < 0 {
			empty = 0
		}
		bar := strings.Repeat("|", filled) + strings.Repeat("-", empty)
		lines = append(lines, bar)
		// Core / Threat
		lines = append(lines, fmt.Sprintf("CORE %d%%  THREAT: %s", obs.Dungeon.CoreIntegrity, obs.Dungeon.Threat))
		lines = append(lines, strings.Repeat("-", 7))
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
					b.WriteRune(g)
				}
				line := b.String()
				// pressure jitter: shift grid one char on odd ticks when pressure >= 19
				if obs.Dungeon.Pressure >= 19 && obs.Tick%2 == 1 {
					line = " " + line
				}
				lines = append(lines, line)
			}
		} else {
			lines = append(lines, "(no grid)")
		}
		lines = append(lines, strings.Repeat("-", 7))
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
	// narration: at most one subtle line
	narration := make([]string, 0, 1)
	if obs.Mode == "dungeon" {
		if obs.Dungeon != nil {
			switch obs.Dungeon.InstabilityBand {
			case 1:
				narration = append(narration, "The air feels unstable.")
			case 2:
				narration = append(narration, "The dungeon resists your presence.")
			case 3:
				narration = append(narration, "The dungeon is close to collapse.")
			}
			// Distortion narration when navigation is actively distorted.
			if obs.Dungeon.DistortionActive {
				narration = append(narration, "Your sense of direction warps for a moment.")
			}
			// Present action-cost hints derived from server-provided metadata.
			if len(obs.Dungeon.ActionCosts) > 0 {
				if v, ok := obs.Dungeon.ActionCosts["observe"]; ok && v > 0 {
					narration = append(narration, "OBSERVE costs more energy here.")
				}
				if v, ok := obs.Dungeon.ActionCosts["move"]; ok && v > 0 {
					narration = append(narration, "Movement feels draining.")
				}
			}
			for _, b := range obs.Dungeon.BlockedActions {
				if b == "wait:norest" || b == "wait:blocked" {
					narration = append(narration, "No rest here.")
				}
				if b == "exit:exhausted" {
					narration = append(narration, "You are too exhausted to exit.")
				}
				if b == "exit:not_at_exit" {
					// subtle hint only
					narration = append(narration, "You must reach the exit to leave.")
				}
				if b == "exit:objective_incomplete" {
					narration = append(narration, "Objective incomplete.")
				}
			}

				// Channeling exit hint
				if obs.Dungeon.ExitChanneling {
					narration = append(narration, "CHANNELING EXIT...")
				}
			// One-shot server events (e.g., eject) will be appended globally below.
		}
	} else if !opts.Minimal && len(obs.Presence) > 0 {
		narration = append(narration, "You sense presences nearby.")
	}

	// Global one-shot event (deliver for any mode)
	if obs.Event != "" {
		narration = append(narration, obs.Event)
	}

	// HUD (compact)
	hud := make([]string, 0, 2)
	hudLabel := color("2;36", "Energy")
	energyStr := fmt.Sprintf(" %d/%d", energy, 100)
	if energy < 30 {
		energyStr = color("1;33", energyStr)
	}
	hud = append(hud, fmt.Sprintf("%s%s", hudLabel, energyStr))
	// Append threat line for dungeon mode when present
	hud = append(hud, fmt.Sprintf("P %d  S %d  Beliefs %d", paranoia, scars, len(obs.Known)))
	if obs.Mode == "dungeon" && obs.Dungeon != nil {
		th := obs.Dungeon.Threat
		if th == "" {
			th = "LOW"
		}
		hud = append(hud, fmt.Sprintf("THREAT: %s", th))
	}

	// HUD cue: enemy locked onto you when any visible enemy has an active lock
	if obs.Mode == "dungeon" && obs.Dungeon != nil {
		for _, ev := range obs.Dungeon.Enemies {
			if ev.TargetLocked {
				hud = append(hud, "LOCKED")
				break
			}
		}
	}

	statusLines := splitStatusLines(status)

	prompt := "> "
	if opts.Minimal {
		prompt = "> "
	}

	header := "WORLD  " + ANSIGrayDim + fmt.Sprintf("tick %d", obs.Tick) + ANSIReset
	if obs.Mode == "dungeon" {
		// Show enraged header if present
		if obs.Dungeon != nil && obs.Dungeon.Enraged {
			// alternate bold every other tick deterministically
			colored := ANSIRedBright + "CRITICAL – ENRAGED" + ANSIReset
			if obs.Tick%2 == 1 {
				colored = "\x1b[1m" + colored + ANSIReset
			}
			header = fmt.Sprintf("DUNGEON  tick %d  [%s]", obs.Tick, colored)
		} else {
			band := 0
			if obs.Dungeon != nil {
				band = obs.Dungeon.InstabilityBand
			}
			label, colored := instabilityLabel(band)
			header = fmt.Sprintf("DUNGEON  tick %d  [%s]", obs.Tick, colored)
			_ = label
		}
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
