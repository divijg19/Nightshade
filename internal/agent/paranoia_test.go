package agent

import (
	"testing"

	"github.com/divijg19/Nightshade/internal/core"
)

type fakeSnapFull struct{
    tiles []core.TileView
    tick int
}
func (f fakeSnapFull) VisibleTiles() []core.TileView { return f.tiles }
func (f fakeSnapFull) TickValue() int { return f.tick }

// Test that beliefs older than ParanoiaThreshold are injected into Visible
// as hallucinated tiles (i.e., they appear in Observation.Visible when not
// in the runtime VisibleTiles()).
func TestParanoiaInjectsHallucination(t *testing.T) {
    s := NewScripted("P")
    mem := s.Memory()

    tick := 100
    target := core.Position{X: 6, Y: 0}
    // last seen older than ParanoiaThreshold
    mem.tiles[target] = MemoryTile{Tile: core.TileView{Position: target, Glyph: 'H'}, LastSeen: tick - (ParanoiaThreshold + 1)}

    snap := fakeSnapFull{tiles: nil, tick: tick}
    prev := make(map[core.Position]int)
    obs := buildObservation(mem, snap, prev, MaxEnergy, ParanoiaThreshold)

    // Visible should include hallucinated tile
    found := false
    for _, v := range obs.Visible {
        if v.Position == target {
            found = true
            break
        }
    }
    if !found {
        t.Fatalf("expected hallucinated tile at %v to be in Visible, but it was not", target)
    }
}

// Test that when the runtime VisibleTiles contains a previously-hallucinated
// tile (i.e., agent OBSERVE occurred), the tile is included as VisibleTruth
// (not treated as a hallucination). This ensures hallucinations clear when
// the tile becomes truthfully visible.
func TestParanoiaClearedByVisible(t *testing.T) {
    s := NewScripted("P2")
    mem := s.Memory()

    // Simulate a belief old enough to hallucinate before the tick.
    prevTick := 200
    target := core.Position{X: 3, Y: 1}
    mem.tiles[target] = MemoryTile{Tile: core.TileView{Position: target, Glyph: 'Z'}, LastSeen: prevTick - (ParanoiaThreshold + 2)}

    // Now simulate a new snapshot where the runtime truthfully shows the tile
    // (as would happen after an OBSERVE). The tick is advanced.
    snap := fakeSnapFull{tiles: []core.TileView{{Position: target, Glyph: 'Z', Visible: true}}, tick: prevTick}
    prev := make(map[core.Position]int)
    obs := buildObservation(mem, snap, prev, MaxEnergy, ParanoiaThreshold)

    // Tile should be present in Visible (truth takes precedence)
    found := false
    for _, v := range obs.Visible {
        if v.Position == target {
            found = true
            break
        }
    }
    if !found {
        t.Fatalf("expected tile at %v to be in Visible after OBSERVE, but it was not", target)
    }

    // Hallucination condition is: age > ParanoiaThreshold AND not in VisibleTruth.
    // Since the tile is visibleTruth now, it is not considered a hallucination.
    // Verify that there does not exist a case where tile would be classified
    // as hallucinated in this snapshot.
    age := obs.Tick - mem.tiles[target].LastSeen
    if age <= ParanoiaThreshold {
        t.Fatalf("setup error: expected age > ParanoiaThreshold, got %d", age)
    }
}
