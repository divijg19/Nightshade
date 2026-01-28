package agent

import (
	"testing"

	"github.com/divijg19/Nightshade/internal/core"
)

// reuse fakeSnapPos from caution_test.go

func TestEnergyDecreaseOnMove(t *testing.T) {
	s := NewScripted("E1")
	start := s.energy
	snap := fakeSnapPos{pos: core.Position{X: 0, Y: 0}, tick: 10}
	_ = s.Decide(snap)
	if s.energy != start-MoveEnergyCost {
		t.Fatalf("expected energy %d after move, got %d", start-MoveEnergyCost, s.energy)
	}
}

func TestEnergyRestoreOnWaitAndClamp(t *testing.T) {
	s := NewScripted("E2")
	s.energy = MaxEnergy - 1
	// force WAIT by setting energy below critical
	s.energy = CriticalEnergyThreshold - 1
	snap := fakeSnapPos{pos: core.Position{X: 0, Y: 0}, tick: 10}
	act := s.Decide(snap)
	if act != WAIT {
		t.Fatalf("expected WAIT due to critical energy, got %v", act)
	}
	if s.energy <= CriticalEnergyThreshold-1 {
		t.Fatalf("expected energy to increase on WAIT, got %d", s.energy)
	}
	// test clamp at MaxEnergy
	s.energy = MaxEnergy - 1
	s.Decide(snap) // no guarantee what action but WAIT restores if forced
	if s.energy > MaxEnergy {
		t.Fatalf("energy exceeded MaxEnergy: %d", s.energy)
	}
}

func TestLowEnergyAcceleratesParanoia(t *testing.T) {
	s := NewScripted("E3")
	mem := s.Memory()
	target := core.Position{X: 5, Y: 5}
	tick := 100
	// set LastSeen such that age = ParanoiaThreshold - 1 (normally not hallucinate)
	mem.tiles[target] = MemoryTile{Tile: core.TileView{Position: target}, LastSeen: tick - (ParanoiaThreshold - 1)}
	// low energy
	s.energy = LowEnergyThreshold - 1
	// Build observation directly
	posSnap := fakeSnapPos{pos: core.Position{X: 0, Y: 0}, tick: tick}
	prev := make(map[core.Position]MemoryTile)
	obs := buildObservation(mem, posSnap, prev, s.energy, ParanoiaThreshold-2)
	found := false
	for _, v := range obs.Visible {
		if v.Position == target {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected hallucination under low energy, but not found")
	}
}

func TestCriticalEnergyForcesWait(t *testing.T) {
	s := NewScripted("E4")
	s.energy = CriticalEnergyThreshold - 1
	snap := fakeSnapPos{pos: core.Position{X: 0, Y: 0}, tick: 10}
	act := s.Decide(snap)
	if act != WAIT {
		t.Fatalf("expected WAIT when critical, got %v", act)
	}
}

func TestObserveFailsToClearAtCriticalEnergy(t *testing.T) {
	s := NewScripted("E5")
	mem := s.Memory()
	target := core.Position{X: 2, Y: 2}
	prevTick := 50
	// store old LastSeen old enough to hallucinate
	mem.tiles[target] = MemoryTile{Tile: core.TileView{Position: target}, LastSeen: prevTick - (ParanoiaThreshold + 2)}

	// snapshot shows tile (OBSERVE scenario). UpdateFromVisible should update LastSeen
	snap := fakeSnap{tiles: []core.TileView{{Position: target, Visible: true}}, tick: prevTick}
	// call UpdateFromVisible and capture prev map
	prev := mem.UpdateFromVisible(snap)
	if mem.tiles[target].LastSeen != prevTick {
		t.Fatalf("expected memory updated to tick %d, got %d", prevTick, mem.tiles[target].LastSeen)
	}

	// Now build observation with critical energy; buildObservation should still treat prev as hallucinated
	s.energy = CriticalEnergyThreshold - 1
	obs := buildObservation(mem, snap, prev, s.energy, ParanoiaThreshold)
	found := false
	for _, v := range obs.Visible {
		if v.Position == target {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected hallucination to persist at critical energy, but not found")
	}
}

func TestRecoveryViaWaitRestoresCognition(t *testing.T) {
	s := NewScripted("E6")
	mem := s.Memory()
	target := core.Position{X: 7, Y: 7}
	tick := 200
	mem.tiles[target] = MemoryTile{Tile: core.TileView{Position: target}, LastSeen: tick - (ParanoiaThreshold + 2)}

	// critical energy first -> hallucinate
	s.energy = CriticalEnergyThreshold - 1
	prev := make(map[core.Position]MemoryTile)
	posSnap := fakeSnapPos{pos: core.Position{X: 0, Y: 0}, tick: tick}
	obs1 := buildObservation(mem, posSnap, prev, s.energy, ParanoiaThreshold)
	found1 := false
	for _, v := range obs1.Visible {
		if v.Position == target {
			found1 = true
			break
		}
	}
	if !found1 {
		t.Fatalf("expected hallucination at critical energy")
	}

	// WAIT to recover energy
	s.energy = CriticalEnergyThreshold - 1
	snap := fakeSnapPos{pos: core.Position{X: 0, Y: 0}, tick: tick + 1}
	act := s.Decide(snap)
	if act != WAIT {
		t.Fatalf("expected WAIT during recovery, got %v", act)
	}
	t.Logf("energy after first WAIT: %d", s.energy)

	// simulate WAITs to restore energy above critical by forcing WAIT each iteration
	for i := 0; i < 20; i++ {
		s.energy = CriticalEnergyThreshold - 1
		s.Decide(snap)
	}
	if s.energy <= CriticalEnergyThreshold {
		t.Fatalf("expected energy restored above critical, got %d", s.energy)
	}

	// simulate OBSERVE after recovery: runtime shows the tile and memory updates
	snapVisible := fakeSnap{tiles: []core.TileView{{Position: target, Visible: true}}, tick: tick + 2}
	prev2 := mem.UpdateFromVisible(snapVisible)
	if mem.tiles[target].LastSeen != tick+2 {
		t.Fatalf("expected memory updated after OBSERVE, got %d", mem.tiles[target].LastSeen)
	}
	// build observation with restored energy - hallucination injection should not use prev2
	obs2 := buildObservation(mem, snapVisible, prev2, s.energy, ParanoiaThreshold)
	// Known age for the tile should be zero after update
	knownAgeZero := false
	for _, k := range obs2.Known {
		if k.Tile.Position == target && k.Age == 0 {
			knownAgeZero = true
			break
		}
	}
	if !knownAgeZero {
		t.Fatalf("expected known age to be 0 after OBSERVE, but it was not")
	}
}
