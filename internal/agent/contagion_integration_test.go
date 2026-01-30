package agent

import (
	"testing"

	"github.com/divijg19/Nightshade/internal/core"
)

func TestApplyBeliefContagion_BasicTransfer(t *testing.T) {
    senderMem := NewMemory()
    receiverMem := NewMemory()

    tilePos := core.Position{X: 5, Y: 5}
    senderMem.SetMemoryTile(tilePos, MemoryTile{Tile: core.TileView{Position: tilePos, Glyph: 'Z', Visible: true}, LastSeen: 10})

    // Emit a belief signal for sender at tick 10
    beliefs := []Belief{{Tile: core.TileView{Position: tilePos, Glyph: 'Z', Visible: true}, Age: 0}}
    emitBeliefSignal("sender", 10, core.Position{X: 0, Y: 0}, beliefs)

    // Receiver at position within BeliefRadius of sender
    applied := applyBeliefContagion("receiver", core.Position{X: 1, Y: 0}, 10, receiverMem, MaxEnergy)
    if len(applied) == 0 {
        t.Fatalf("expected contagion to apply, none applied")
    }
    if mt, ok := receiverMem.GetMemoryTile(tilePos); !ok {
        t.Fatalf("receiver did not acquire tile")
    } else {
        if mt.Tile.Position != tilePos {
            t.Fatalf("tile position mismatch: %+v", mt.Tile.Position)
        }
    }
}
