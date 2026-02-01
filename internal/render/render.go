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
	ANSIReset       = "\x1b[0m"
	ANSIWhiteBright = "\x1b[97m"
	ANSICyan        = "\x1b[36m"
	ANSIYellow      = "\x1b[33m"
	ANSIGrayDim     = "\x1b[90m"
	ANSIMagentaDim  = "\x1b[2;35m"
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
	Grid      []string
	Narration []string
	Status    []string
	HUD       []string
	Prompt    string
}

// RenderGrid returns fixed-size, colorized grid lines for the observation.
// It does NOT perform any IO.
func RenderGrid(obs agent.Observation) []string {
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
	// narration: a single subtle line
	narration := make([]string, 0, 1)
	if !opts.Minimal && len(obs.Presence) > 0 {
		narration = append(narration, "You sense presences nearby.")
	}

	// HUD (compact)
	hud := make([]string, 0, 2)
	hudLabel := color("2;36", "Energy")
	energyStr := fmt.Sprintf(" %d/%d", energy, 100)
	if energy < 30 {
		energyStr = color("1;33", energyStr)
	}
	hud = append(hud, fmt.Sprintf("%s%s", hudLabel, energyStr))
	hud = append(hud, fmt.Sprintf("P %d  S %d  Beliefs %d", paranoia, scars, len(obs.Known)))

	statusLines := splitStatusLines(status)

	prompt := "> "
	if opts.Minimal {
		prompt = "> "
	}

	return Frame{
		Tick:      obs.Tick,
		Options:   opts,
		Grid:      grid,
		Narration: narration,
		Status:    statusLines,
		HUD:       hud,
		Prompt:    prompt,
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
	if frameW > 60 {
		frameW = 60
	}
	sep := strings.Repeat("-", frameW)

	// Header + single separator
	writeLine(&sb, "WORLD  "+ANSIGrayDim+fmt.Sprintf("tick %d", f.Tick)+ANSIReset)
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
