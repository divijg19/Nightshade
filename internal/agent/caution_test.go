package agent

import (
	"testing"

	"github.com/divijg19/Nightshade/internal/core"
)

type fakeSnapPos struct {
	pos  core.Position
	tick int
}

func (f fakeSnapPos) VisibleTiles() []core.TileView { return nil }
func (f fakeSnapPos) TickValue() int                { return f.tick }
func (f fakeSnapPos) PositionValue() core.Position  { return f.pos }

func TestScriptedCautionObservesWhenStale(t *testing.T) {
	s := NewScripted("T")
	// place agent at (0,0), intended MOVE_E target is (1,0)
	tick := 10
	target := core.Position{X: 1, Y: 0}
	// record a stale observation older than threshold
	s.memory.tiles[target] = MemoryTile{Tile: core.TileView{Position: target}, LastSeen: tick - (CautionThreshold + 1)}

	snap := fakeSnapPos{pos: core.Position{X: 0, Y: 0}, tick: tick}
	act := s.Decide(snap)
	if act != OBSERVE {
		t.Fatalf("expected OBSERVE due to stale belief, got %v", act)
	}

	// re-observe at tick -> should allow move (i.e., not OBSERVE)
	s.memory.tiles[target] = MemoryTile{Tile: core.TileView{Position: target}, LastSeen: tick}
	act2 := s.Decide(snap)
	if act2 == OBSERVE {
		t.Fatalf("expected move allowed after re-observation, got OBSERVE")
	}
}

func TestOscillatingCaution(t *testing.T) {
	o := NewOscillating("O")
	// place agent at (0,0). On odd tick oscillator moves south, even moves north.
	// Pick an even tick to intend MOVE_N -> target (0,-1) which is out of bounds,
	// so pick odd tick -> MOVE_S target (0,1)
	tick := 11 // odd -> MOVE_S
	target := core.Position{X: 0, Y: 1}
	o.memory.tiles[target] = MemoryTile{Tile: core.TileView{Position: target}, LastSeen: tick - (CautionThreshold + 2)}

	snap := fakeSnapPos{pos: core.Position{X: 0, Y: 0}, tick: tick}
	act := o.Decide(snap)
	if act != OBSERVE {
		t.Fatalf("expected OBSERVE for Oscillating due to stale belief, got %v", act)
	}

	// refresh observation -> should move (i.e., not OBSERVE)
	o.memory.tiles[target] = MemoryTile{Tile: core.TileView{Position: target}, LastSeen: tick}
	act2 := o.Decide(snap)
	if act2 == OBSERVE {
		t.Fatalf("expected move after re-observation, got OBSERVE")
	}
}
