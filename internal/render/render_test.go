package render

import (
	"testing"

	"strings"

	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/core"
)

func TestRender_ClearsScreen(t *testing.T) {
	obs := agent.Observation{
		Visible:  []core.TileView{{Position: core.Position{X: 0, Y: 0}, Glyph: 0, Visible: true}},
		Known:    nil,
		Tick:     0,
		Position: core.Position{X: 0, Y: 0},
	}
	out := RenderForTest(obs)
	if out == "" {
		t.Fatalf("render output empty")
	}
	if len(out) < 2 {
		t.Fatalf("render output too small")
	}
	if !strings.HasPrefix(out, clearScreen+cursorHome) {
		t.Fatalf("render output does not start with clear+home")
	}
	if !strings.Contains(out, "\r\n") {
		t.Fatalf("render output missing CRLF line endings")
	}
}

func TestPresenceRendering(t *testing.T) {
	center := core.Position{X: 0, Y: 0}
	obs := agent.Observation{
		Visible:  nil,
		Known:    nil,
		Tick:     0,
		Position: center,
		Presence: nil,
	}
	out := RenderForTest(obs)
	if !strings.Contains(out, ANSIWhiteBright+"@"+ANSIReset) {
		t.Fatalf("self not rendered as bright white @")
	}

	// HumanOther presence to the east
	obs2 := obs
	obs2.Presence = []agent.PresenceCue{{Type: agent.PresenceHumanOther, Position: core.Position{X: 1, Y: 0}}}
	out2 := RenderForTest(obs2)
	if !strings.Contains(out2, ANSICyan+"@"+ANSIReset) {
		t.Fatalf("human other not rendered as cyan @")
	}

	// NPC presence to the west
	obs3 := obs
	obs3.Presence = []agent.PresenceCue{{Type: agent.PresenceNPC, Position: core.Position{X: -1, Y: 0}}}
	out3 := RenderForTest(obs3)
	if !strings.Contains(out3, ANSIGrayDim+"@"+ANSIReset) {
		t.Fatalf("npc not rendered as gray dim @")
	}
}

func TestPresenceOverlapPriority(t *testing.T) {
	center := core.Position{X: 0, Y: 0}
	obs := agent.Observation{
		Visible:  nil,
		Known:    nil,
		Tick:     0,
		Position: center,
	}
	// same position has NPC and HumanOther: expect HumanOther (cyan)
	obs.Presence = []agent.PresenceCue{
		{Type: agent.PresenceNPC, Position: core.Position{X: 1, Y: 1}},
		{Type: agent.PresenceHumanOther, Position: core.Position{X: 1, Y: 1}},
	}
	out := RenderForTest(obs)
	if !strings.Contains(out, ANSICyan+"@"+ANSIReset) {
		t.Fatalf("overlap priority failed: expected cyan @")
	}
	if strings.Contains(out, ANSIGrayDim+"@"+ANSIReset) {
		t.Fatalf("overlap priority failed: gray @ should be suppressed by human other")
	}
}

func TestRender_IdempotentAndLayout(t *testing.T) {
	center := core.Position{X: 0, Y: 0}
	obs := agent.Observation{
		Visible:  nil,
		Known:    nil,
		Tick:     0,
		Position: center,
	}
	out1 := RenderForTest(obs)
	out2 := RenderForTest(obs)
	if out1 != out2 {
		t.Fatalf("render not idempotent: outputs differ")
	}

	// layout checks: WORLD header, narration line, HUD label, prompt
	if !strings.Contains(out1, "WORLD") {
		t.Fatalf("missing WORLD VIEW header")
	}
	if !strings.Contains(out1, "Energy") {
		t.Fatalf("missing HUD Energy label")
	}
	if !strings.Contains(out1, "> ") {
		t.Fatalf("missing prompt")
	}
}

func TestEpistemicColoring(t *testing.T) {
	center := core.Position{X: 0, Y: 0}
	// Visible tile at (0,1)
	vis := core.TileView{Position: core.Position{X: 0, Y: 1}, Glyph: '.', Visible: true}
	obs := agent.Observation{
		Visible:  []core.TileView{vis},
		Known:    nil,
		Tick:     0,
		Position: center,
	}
	out := RenderForTest(obs)
	if !strings.Contains(out, ANSIWhiteBright+"."+ANSIReset) {
		t.Fatalf("visible tile not white")
	}

	// Believed tile at (1,0)
	b := agent.Belief{Tile: core.TileView{Position: core.Position{X: 1, Y: 0}, Glyph: 'b'}, Age: 1}
	obs2 := obs
	obs2.Visible = nil
	obs2.Known = []agent.Belief{b}
	out2 := RenderForTest(obs2)
	if !strings.Contains(out2, ANSIGrayDim+"b"+ANSIReset) {
		t.Fatalf("believed tile not dim gray")
	}

	// Hallucinated: visible + known age > ParanoiaThreshold
	hv := core.TileView{Position: core.Position{X: -1, Y: 0}, Glyph: 'H', Visible: true}
	hb := agent.Belief{Tile: core.TileView{Position: core.Position{X: -1, Y: 0}, Glyph: 'H'}, Age: agent.ParanoiaThreshold + 1}
	obs3 := agent.Observation{Visible: []core.TileView{hv}, Known: []agent.Belief{hb}, Tick: 0, Position: center}
	out3 := RenderForTest(obs3)
	if !strings.Contains(out3, ANSIMagentaDim+"H"+ANSIReset) {
		t.Fatalf("hallucinated tile not magenta")
	}
}

func TestDungeonHeader_LabelAndColor(t *testing.T) {
	baseGrid := [][]rune{
		[]rune("#######"),
		[]rune("#.....#"),
		[]rune("#.....#"),
		[]rune("#..A..#"),
		[]rune("#.....#"),
		[]rune("#.....#"),
		[]rune("###X###"),
	}

	cases := []struct {
		band        int
		label       string
		wantColor   string
		shouldColor bool
	}{
		{band: 0, label: "STABLE", wantColor: "", shouldColor: false},
		{band: 1, label: "UNSTABLE", wantColor: ANSIYellowDim, shouldColor: true},
		{band: 2, label: "DANGEROUS", wantColor: ANSIYellowBright, shouldColor: true},
		{band: 3, label: "CRITICAL", wantColor: ANSIRedBright, shouldColor: true},
	}

	for _, tc := range cases {
		obs := agent.Observation{
			Tick:     42,
			Position: core.Position{X: 0, Y: 0},
			Mode:     "dungeon",
			Dungeon: &agent.DungeonView{
				Grid:            baseGrid,
				Pressure:        0,
				MaxPressure:     20,
				Tick:            42,
				InstabilityBand: tc.band,
			},
		}
		out := RenderForTest(obs)
		if !strings.Contains(out, "DUNGEON  tick 42") {
			t.Fatalf("missing dungeon header")
		}
		if !strings.Contains(out, "["+tc.label+"]") {
			// stable has no coloring, others have ANSI wrapped; allow either.
			if tc.shouldColor {
				if !strings.Contains(out, "["+tc.wantColor+tc.label+ANSIReset+"]") {
					t.Fatalf("band=%d missing colored label %q", tc.band, tc.label)
				}
			} else {
				t.Fatalf("band=%d missing plain label %q", tc.band, tc.label)
			}
		}
		if tc.shouldColor {
			if !strings.Contains(out, tc.wantColor+tc.label+ANSIReset) {
				t.Fatalf("band=%d missing ANSI color code", tc.band)
			}
		}
	}
}

func TestDungeonGrid_SubstitutionDeterministic(t *testing.T) {
	grid := [][]rune{
		[]rune("#######"),
		[]rune("#.....#"),
		[]rune("#.....#"),
		[]rune("#..A..#"),
		[]rune("#.....#"),
		[]rune("#.....#"),
		[]rune("###X###"),
	}

	obs := agent.Observation{
		Tick:     0,
		Position: core.Position{X: 0, Y: 0},
		Mode:     "dungeon",
		Dungeon: &agent.DungeonView{
			Grid:            grid,
			Pressure:        12,
			MaxPressure:     20,
			Tick:            0,
			InstabilityBand: 2,
		},
	}
	out := RenderForTest(obs)

	// Band>=2 wall flicker: row0 y=0 with (x+y+tick)%3==0 => x=0,3,6 => "%##%##%"
	if !strings.Contains(out, "%##%##%") {
		t.Fatalf("expected wall flicker substitution in output")
	}
	// Band>=1 floor wobble: row1 y=1 with (x+y+tick)%4==0 => x=3 => "#..~..#"
	if !strings.Contains(out, "#..~..#") {
		t.Fatalf("expected floor wobble substitution in output")
	}

	// Deterministic: rendering same obs twice is identical.
	out2 := RenderForTest(obs)
	if out != out2 {
		t.Fatalf("expected deterministic render output")
	}
}
