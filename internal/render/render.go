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

// Frame represents a full UI frame assembled in memory.
type Frame struct {
	Grid      []string
	Narration []string
	Ephemeral string
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
func BuildFrame(obs agent.Observation, ephemeral string, energy int, paranoia int, scars int) Frame {
	grid := RenderGrid(obs)
	// narration: up to 2 lines
	narration := make([]string, 0, 2)
	if len(obs.Presence) > 0 {
		narration = append(narration, "You sense presences nearby.")
	} else {
		narration = append(narration, "")
	}
	// second narration line reserved
	narration = append(narration, "")

	// HUD
	hud := make([]string, 0, 2)
	hudLabel := color("2;36", "Energy:")
	energyStr := fmt.Sprintf(" %d/%d", energy, 100)
	if energy < 30 {
		energyStr = color("1;33", energyStr)
	}
	hudLine := fmt.Sprintf("%s%s  Paranoia: %d  Scars: %d  Beliefs: %d", hudLabel, energyStr, paranoia, scars, len(obs.Known))
	hud = append(hud, hudLine)

	return Frame{
		Grid:      grid,
		Narration: narration,
		Ephemeral: ephemeral,
		HUD:       hud,
		Prompt:    "> ",
	}
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

	// Fixed separator width (consistent frame width)
	const fixedWidth = 40
	ansiRE := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	sepLen := fixedWidth
	sep := strings.Repeat("-", sepLen)
	writeLine(&sb, sep)
	writeLine(&sb, "WORLD")
	writeLine(&sb, sep)

	// Grid lines
	for _, line := range f.Grid {
		vis := ansiRE.ReplaceAllString(line, "")
		visLen := len([]rune(vis))
		if visLen < sepLen {
			line = line + strings.Repeat(" ", sepLen-visLen)
		} else if visLen > sepLen {
			// If the visible content exceeds fixed width, truncate the visible
			// portion but keep ANSI sequences omitted (rare for 5x5 grid).
			runes := []rune(ansiRE.ReplaceAllString(line, ""))
			truncated := string(runes[:sepLen])
			line = truncated
		}
		writeLine(&sb, line)
	}

	writeLine(&sb, sep)
	// Narration: exactly two lines (pad if empty)
	for i := 0; i < 2; i++ {
		if i < len(f.Narration) {
			writeLine(&sb, f.Narration[i])
		} else {
			writeLine(&sb, "")
		}
	}
	// Ephemeral
	if f.Ephemeral != "" {
		writeLine(&sb, f.Ephemeral)
	} else {
		writeLine(&sb, "")
	}

	writeLine(&sb, sep)
	// HUD lines
	for _, h := range f.HUD {
		vis := ansiRE.ReplaceAllString(h, "")
		if len([]rune(vis)) < sepLen {
			h = h + strings.Repeat(" ", sepLen-len([]rune(vis)))
		}
		writeLine(&sb, h)
	}
	writeLine(&sb, sep)
	// Prompt line
	sb.WriteString(f.Prompt)

	// flush buffer to writer
	_, _ = io.WriteString(w, sb.String())
}

// RenderTo kept for external callers; builds frame and writes it.
func RenderTo(w io.Writer, obs agent.Observation, energy int, paranoia int, scars int, prompt string, ephemeral string) {
	f := BuildFrame(obs, ephemeral, energy, paranoia, scars)
	// ensure prompt uses provided prompt string appended to default prompt
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
