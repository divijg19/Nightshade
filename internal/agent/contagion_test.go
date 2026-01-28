package agent

import (
	"testing"

	"github.com/divijg19/Nightshade/internal/core"
)

// Test that belief transfers in range and applies TransferPenalty
func TestBeliefTransfersInRange(t *testing.T) {
    a := NewScripted("A")
    b := NewScripted("B")
    // place A at (0,0), B at (1,0) within BeliefRadius=1
    aPos := core.Position{X:0, Y:0}
    bPos := core.Position{X:1, Y:0}
    // A remembers tile t at LastSeen such that Age small
    tick := 100
    tilePos := core.Position{X:5, Y:5}
    a.Memory().tiles[tilePos] = MemoryTile{Tile: core.TileView{Position: tilePos}, LastSeen: tick}

    // Emit A signal
    emitBeliefSignal(a.ID(), tick, aPos, []Belief{{Tile: core.TileView{Position: tilePos}, Age: 0}})
    // Apply contagion to B
    applied := applyBeliefContagion(b.ID(), bPos, tick, b.Memory(), MaxEnergy)
    if len(applied) == 0 {
        t.Fatalf("expected belief to transfer in range")
    }
    mt, ok := b.Memory().GetMemoryTile(tilePos)
    if !ok {
        t.Fatalf("expected B to remember transferred tile")
    }
    if mt.LastSeen != tick-TransferPenalty {
        t.Fatalf("expected LastSeen=%d got %d", tick-TransferPenalty, mt.LastSeen)
    }
}

// Test that belief does not transfer out of range
func TestBeliefDoesNotTransferOutOfRange(t *testing.T) {
    a := NewScripted("A2")
    b := NewScripted("B2")
    aPos := core.Position{X:0, Y:0}
    bPos := core.Position{X:3, Y:0}
    tick := 100
    tilePos := core.Position{X:8, Y:8}
    a.Memory().tiles[tilePos] = MemoryTile{Tile: core.TileView{Position: tilePos}, LastSeen: tick}
    emitBeliefSignal(a.ID(), tick, aPos, []Belief{{Tile: core.TileView{Position: tilePos}, Age: 0}})
    applied := applyBeliefContagion(b.ID(), bPos, tick, b.Memory(), MaxEnergy)
    if len(applied) != 0 {
        t.Fatalf("expected no transfer out of range")
    }
}
