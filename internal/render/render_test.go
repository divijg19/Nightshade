package render

import (
	"testing"

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
