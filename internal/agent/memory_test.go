package agent

import (
	"testing"

	"github.com/divijg19/Nightshade/internal/core"
)

type fakeSnap struct {
	tiles []core.TileView
	tick  int
}

func (f fakeSnap) VisibleTiles() []core.TileView { return f.tiles }
func (f fakeSnap) TickValue() int                { return f.tick }

func TestMemoryLastSeenUpdates(t *testing.T) {
	mem := NewMemory()
	pos := core.Position{X: 1, Y: 1}
	tv := core.TileView{Position: pos, Glyph: 'x', Visible: true}

	// Observe at tick 10
	mem.UpdateFromVisible(fakeSnap{tiles: []core.TileView{tv}, tick: 10})
	mts := mem.All()
	if len(mts) != 1 {
		t.Fatalf("expected 1 remembered tile, got %d", len(mts))
	}
	if mts[0].LastSeen != 10 {
		t.Fatalf("expected LastSeen 10, got %d", mts[0].LastSeen)
	}

	// Observe the same tile at tick 15 -> LastSeen should update
	tv2 := core.TileView{Position: pos, Glyph: 'y', Visible: true}
	mem.UpdateFromVisible(fakeSnap{tiles: []core.TileView{tv2}, tick: 15})
	mts2 := mem.All()
	if len(mts2) != 1 {
		t.Fatalf("expected 1 remembered tile, got %d", len(mts2))
	}
	if mts2[0].LastSeen != 15 {
		t.Fatalf("expected LastSeen 15 after re-observation, got %d", mts2[0].LastSeen)
	}
	if mts2[0].Tile.Glyph != 'y' {
		t.Fatalf("expected Tile to be overwritten on re-observation")
	}
}
