package render

import (
	"reflect"
	"testing"

	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/core"
)

// Ensure rendering does not mutate the provided Observation (purity).
func TestRender_Purity_NoMutation(t *testing.T) {
    grid := [][]rune{
        []rune("#######"),
        []rune("#.....#"),
        []rune("#..A..#"),
        []rune("#.....#"),
        []rune("###X###"),
    }
    obs := agent.Observation{
        Visible: []core.TileView{{Position: core.Position{X: 0, Y: 0}, Glyph: '.', Visible: true}},
        Known:   nil,
        Tick:    7,
        Position: core.Position{X: 0, Y: 0},
        Mode:    "dungeon",
        Dungeon: &agent.DungeonView{Grid: grid, Pressure: 5, MaxPressure: 20, Tick: 7, InstabilityBand: 1},
    }

    // Make a deep copy
    before := obs
    // Render twice
    _ = RenderForTest(obs)
    _ = RenderForTest(obs)

    if !reflect.DeepEqual(before, obs) {
        t.Fatalf("render mutated observation: before=%#v after=%#v", before, obs)
    }
}
