package render

import (
	"strings"
	"testing"

	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/core"
)

func TestRender_DecayedTileAndDistortionNarration(t *testing.T) {
    grid := [][]rune{
        []rune("#######"),
        []rune("#.....#"),
        []rune("#..A..#"),
        []rune("#.....#"),
        []rune("###X###"),
    }
    obs := agent.Observation{
        Tick: 42,
        Mode: "dungeon",
        Dungeon: &agent.DungeonView{
            Grid:            grid,
            Pressure:        12,
            MaxPressure:     20,
            Tick:            42,
            InstabilityBand: 2,
            DistortionActive: true,
            DecayedTiles: []core.Position{{X:3,Y:2}},
        },
    }
    out := RenderForTest(obs)
    if !strings.Contains(out, "Your sense of direction") && !strings.Contains(out, "warps") {
        // allow variant wording
        if !strings.Contains(out, "Your sense of direction") {
            t.Fatalf("expected distortion narration in render output")
        }
    }
    if !strings.Contains(out, "~") {
        t.Fatalf("expected decayed tile '~' in render output")
    }
}

func TestRender_ForcedEjectEventOneFrame(t *testing.T) {
    obs := agent.Observation{Event: "You are ejected from the dungeon!", Mode: "board"}
    out := RenderForTest(obs)
    if !strings.Contains(out, "ejected") {
        t.Fatalf("expected forced-eject event text in render output")
    }
}
