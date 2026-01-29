package agent

import (
	"testing"

	"github.com/divijg19/Nightshade/internal/core"
)

func TestIntrospectBucketsAndScars(t *testing.T) {
	mem := NewMemory()
	// tick reference
	tick := 100
	// Certain (age 0)
	mem.tiles[core.Position{X:0,Y:0}] = MemoryTile{Tile: core.TileView{Position: core.Position{X:0,Y:0}}, LastSeen: tick}
	// Recent (age 1..CautionThreshold)
	mem.tiles[core.Position{X:1,Y:0}] = MemoryTile{Tile: core.TileView{Position: core.Position{X:1,Y:0}}, LastSeen: tick - 1}
	// Fading (CautionThreshold < age <= ParanoiaThreshold)
	mem.tiles[core.Position{X:2,Y:0}] = MemoryTile{Tile: core.TileView{Position: core.Position{X:2,Y:0}}, LastSeen: tick - (CautionThreshold + 1)}
	// Doubtful (age > ParanoiaThreshold)
	mem.tiles[core.Position{X:3,Y:0}] = MemoryTile{Tile: core.TileView{Position: core.Position{X:3,Y:0}}, LastSeen: tick - (ParanoiaThreshold + 1)}
	// Scar present
	mem.tiles[core.Position{X:4,Y:0}] = MemoryTile{Tile: core.TileView{Position: core.Position{X:4,Y:0}}, LastSeen: tick - 2, ScarLevel: 1}

	rep := Introspect(*mem, tick)
	// Compute expected buckets programmatically to avoid hardcoding off-by-one
	exp := IntrospectionReport{}
	for _, mt := range mem.tiles {
		exp.TotalBeliefs++
		age := tick - mt.LastSeen
		if age == 0 {
			exp.Certain++
		} else if age >= 1 && age <= CautionThreshold {
			exp.Recent++
		} else if age > CautionThreshold && age <= ParanoiaThreshold {
			exp.Fading++
		} else if age > ParanoiaThreshold {
			exp.Doubtful++
		}
		if mt.ScarLevel > 0 { exp.HasScars = true }
	}
	if rep != exp {
		t.Fatalf("expected %v, got %v", exp, rep)
	}
}

func TestIntrospectDoesNotMutateMemory(t *testing.T) {
	mem := NewMemory()
	pos := core.Position{X:7,Y:7}
	mem.tiles[pos] = MemoryTile{Tile: core.TileView{Position: pos}, LastSeen: 10, ScarLevel: 2}
	copyBefore := mem.tiles[pos]
	_ = Introspect(*mem, 20)
	copyAfter := mem.tiles[pos]
	if copyBefore != copyAfter {
		t.Fatalf("Introspect mutated memory: before=%v after=%v", copyBefore, copyAfter)
	}
}
