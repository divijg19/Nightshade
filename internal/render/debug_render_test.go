package render

import (
	"fmt"
	"testing"

	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/core"
)

func TestDebugDungeonGridRender(t *testing.T) {
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
    fmt.Println("---- RAW OUTPUT QUOTED ----")
    fmt.Printf("%q\n", out)
    fmt.Println("---- RAW OUTPUT LINES ----")
    // print lines separately for easier inspection
    f := BuildFrameWithOptions(obs, "", 100, 3, 0, Options{})
    for i, l := range f.Grid {
        fmt.Printf("line %d: %q\n", i, l)
    }
}
