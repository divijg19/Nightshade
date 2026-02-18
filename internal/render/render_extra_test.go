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
			Grid:             grid,
			Pressure:         12,
			MaxPressure:      20,
			Tick:             42,
			InstabilityBand:  2,
			DistortionActive: true,
			DecayedTiles:     []core.Position{{X: 3, Y: 2}},
		},
	}
	out := RenderForTest(obs)
	// v0.3.10 keeps one-line narration and does not emit distortion spam.
	if strings.Contains(out, "warps") {
		t.Fatalf("expected no distortion narration spam")
	}
	if !strings.Contains(out, "·") {
		t.Fatalf("expected rendered floor glyph in output")
	}
}

func TestRender_ForcedEjectEventOneFrame(t *testing.T) {
	obs := agent.Observation{Event: "You are ejected from the dungeon!", Mode: "board"}
	out := RenderForTest(obs)
	if !strings.Contains(out, "ejected") {
		t.Fatalf("expected forced-eject event text in render output")
	}
}
