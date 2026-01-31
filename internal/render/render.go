package render

import (
	"fmt"
	"io"
	"strings"

	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/core"
)

// ANSI control sequences
const (
	esc         = "\x1b"
	clearScreen = esc + "[2J"
	cursorHome  = esc + "[H"
	hideCursor  = esc + "[?25l"
	showCursor  = esc + "[?25h"
)

const (
	ANSIReset       = "\x1b[0m"
	ANSIWhiteBright = "\x1b[97m"
	ANSICyan        = "\x1b[36m"
	ANSIGrayDim     = "\x1b[90m"
)
// minimal color helpers
func color(code string, s string) string { return esc + "[" + code + "m" + s + esc + "[0m" }

// RenderTo draws the full-screen frame to the provided writer. It is
// idempotent: each call emits a full-screen clear and redraw.
func RenderTo(w io.Writer, obs agent.Observation, energy int, paranoia int, scars int, prompt string, ephemeral string) {
	// Clear and home
	fmt.Fprint(w, clearScreen)
	fmt.Fprint(w, cursorHome)
	fmt.Fprint(w, hideCursor)

	// Box header
	fmt.Fprintln(w, "┌──────── WORLD VIEW ────────┐")

	// Render a fixed-size viewport (5x5) centered on obs.Position
	r := 2 // radius
	center := obs.Position
	visMap := map[core.Position]rune{}
	for _, v := range obs.Visible {
		visMap[v.Position] = v.Glyph
	}
	// build presence map (position -> PresenceType) with priority
	presenceMap := map[core.Position]string{}
	for _, p := range obs.Presence {
		// priority resolution: Self > HumanOther > NPC
		cur, ok := presenceMap[p.Position]
		if !ok {
			presenceMap[p.Position] = string(p.Type)
			continue
		}
		// promote if incoming has higher priority
		// map priority numbers: Self=3, HumanOther=2, NPC=1
		prio := func(t string) int {
			switch t {
			case string(agent.PresenceSelf):
				return 3
			case string(agent.PresenceHumanOther):
				return 2
			case string(agent.PresenceNPC):
				return 1
			default:
				return 0
			}
		}
		if prio(string(p.Type)) > prio(cur) {
			presenceMap[p.Position] = string(p.Type)
		}
	}
	for dy := -r; dy <= r; dy++ {
		fmt.Fprint(w, "│")
		for dx := -r; dx <= r; dx++ {
			pos := core.Position{X: center.X + dx, Y: center.Y + dy}
			if dx == 0 && dy == 0 {
				// self always bright white
				fmt.Fprint(w, ANSIWhiteBright+"@"+ANSIReset)
				continue
			}
			// Presence overlay: if a presence cue exists here, draw it
			if pt, ok := presenceMap[pos]; ok {
				switch pt {
				case string(agent.PresenceHumanOther):
					fmt.Fprint(w, ANSICyan+"@"+ANSIReset)
					continue
				case string(agent.PresenceNPC):
					fmt.Fprint(w, ANSIGrayDim+"@"+ANSIReset)
					continue
				case string(agent.PresenceSelf):
					fmt.Fprint(w, ANSIWhiteBright+"@"+ANSIReset)
					continue
				}
			}
			if g, ok := visMap[pos]; ok {
				// marker
				switch g {
				case 'M':
					fmt.Fprint(w, color("0;37", "M"))
				case 0:
					// visible empty
					fmt.Fprint(w, ".")
				default:
					// hallucinated glyphs dim red
					s := string(g)
					// other humans are not in Visible; we cannot show them here
					fmt.Fprint(w, color("2;35", s))
				}
			} else {
				// unknown / fog
				fmt.Fprint(w, color("1;30", "?"))
			}
		}
		fmt.Fprintln(w, "│")
	}

	fmt.Fprintln(w, "└────────────────────────────┘")

	// NARRATION (0-2 lines) — keep nondisclosing and limited
	if len(obs.Presence) > 0 {
		fmt.Fprintln(w, "You sense presences nearby.")
	} else {
		fmt.Fprintln(w, "")
	}

	// Ephemeral single-line feedback (client-only). Appears below narration
	if ephemeral != "" {
		fmt.Fprintln(w, ephemeral)
	} else {
		// pad so layout is stable
		fmt.Fprintln(w, "")
	}

	// HUD line
	hudLabel := color("2;36", "Energy:")
	energyStr := fmt.Sprintf(" %d/%d", energy, 100)
	if energy < 30 {
		energyStr = color("1;33", energyStr)
	}
	fmt.Fprintf(w, "%s%s  Paranoia: %d  Scars: %d  Beliefs: %d\n", hudLabel, energyStr, paranoia, scars, len(obs.Known))

	// Visual separator then prompt (cursor lands here)
	fmt.Fprintln(w, "")
	fmt.Fprint(w, "> ")
	fmt.Fprint(w, prompt)
	fmt.Fprint(w, showCursor)
}

// Helper used by tests to render with a short prompt
func RenderForTest(obs agent.Observation) string {
	var b strings.Builder
	RenderTo(&b, obs, 100, 3, 0, "", "")
	return b.String()
}
