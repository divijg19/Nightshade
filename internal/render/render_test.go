package render

import (
	"testing"

	"strings"

	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/core"
)

func TestRender_ClearsScreen(t *testing.T) {
    obs := agent.Observation{
        Visible: []core.TileView{{Position: core.Position{X: 0, Y: 0}, Glyph: 0, Visible: true}},
        Known:   nil,
        Tick:    0,
        Position: core.Position{X: 0, Y: 0},
    }
    out := RenderForTest(obs)
    if out == "" {
        t.Fatalf("render output empty")
    }
    if len(out) < 2 {
        t.Fatalf("render output too small")
    }
}

func TestPresenceRendering(t *testing.T) {
    center := core.Position{X: 0, Y: 0}
    obs := agent.Observation{
        Visible: nil,
        Known:   nil,
        Tick:    0,
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
        Visible: nil,
        Known:   nil,
        Tick:    0,
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
