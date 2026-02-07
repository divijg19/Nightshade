package agent

import (
	"testing"

	"github.com/divijg19/Nightshade/internal/core"
)

// simpleSnap implements the minimal Snapshot accessors used by DecideWithInput
type simpleSnap struct{ Tick int; Pos core.Position }
func (s simpleSnap) TickValue() int               { return s.Tick }
func (s simpleSnap) PositionValue() core.Position { return s.Pos }

// Verify dungeon scar is applied and decays via OBSERVE outside dungeon.
func TestScar_PersistenceAndObserveDecay(t *testing.T) {
    a := NewRemoteHumanFromExisting("A", NewMemory(), MaxEnergy)

    // Simulate server-applied scar on tile (0,0)
    mt := MemoryTile{Tile: core.TileView{Position: core.Position{X: 0, Y: 0}, Glyph: 'X', Visible: true}, LastSeen: 0, ScarLevel: 2, ScarSource: "dungeon"}
    a.Memory().SetMemoryTile(core.Position{X: 0, Y: 0}, mt)

    // Scar persists across ticks when not observing
    // tick 1: do nothing
    // (we simulate by not calling Decide; directly check memory)
    if m, ok := a.Memory().GetMemoryTile(core.Position{X: 0, Y: 0}); !ok || m.ScarLevel != 2 || m.ScarSource != "dungeon" {
        t.Fatalf("expected scar present with source 'dungeon', got %+v", m)
    }

    // Now simulate an OBSERVE action (which should decrement scar by 1)
    // RemoteHuman.DecideWithInput reduces scar when final == OBSERVE.
    // We call DecideWithInput with input 'e' to simulate OBSERVE.
    snap := simpleSnap{Tick: 1, Pos: core.Position{X: 0, Y: 0}}
    _ = a.DecideWithInput(snap, "e")
    if m2, ok := a.Memory().GetMemoryTile(core.Position{X: 0, Y: 0}); !ok || m2.ScarLevel != 1 {
        t.Fatalf("expected scar to decrement to 1 after OBSERVE, got %+v", m2)
    }
}
