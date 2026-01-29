package agent

import (
	"testing"

	"github.com/divijg19/Nightshade/internal/core"
)

// helper to produce a sequence of inputs
func seq(inputs []string) func() (string, error) {
	i := 0
	return func() (string, error) {
		if i >= len(inputs) { return "", nil }
		s := inputs[i]
		i++
		return s, nil
	}
}

func TestSnapshotsRecordedOnlyAfterActions(t *testing.T) {
	h := NewHuman("R1")
	h.memory.tiles[core.Position{X:0,Y:0}] = MemoryTile{Tile: core.TileView{Position: core.Position{X:0,Y:0}}, LastSeen: 1}
	// input: single WAIT
	HumanInput = seq([]string{"."})
	snap := fakeSnapPos{pos: core.Position{X:0,Y:0}, tick: 10}
	_ = h.Decide(snap)
	if h.snaps.len() != 1 {
		t.Fatalf("expected 1 snapshot after action, got %d", h.snaps.len())
	}
}

func TestReplayNavigationDoesNotMutateState(t *testing.T) {
	h := NewHuman("R2")
	pos := core.Position{X:2,Y:2}
	h.memory.tiles[pos] = MemoryTile{Tile: core.TileView{Position: pos}, LastSeen: 5, ScarLevel: 1}
	initialMemory := h.memory.tiles[pos]
	initialEnergy := h.energy
	// inputs: enter replay '[', move forward ']', then exit with WAIT '.'
	HumanInput = seq([]string{"[", "]", "."})
	snap := fakeSnapPos{pos: core.Position{X:0,Y:0}, tick: 20}
	_ = h.Decide(snap)
	// memory should be unchanged (no mutation during replay)
	after := h.memory.tiles[pos]
	if after != initialMemory {
		t.Fatalf("memory mutated during replay: before=%v after=%v", initialMemory, after)
	}
	// energy should only reflect the final action (WAIT restores energy)
	expectedEnergy := initialEnergy + WaitEnergyRestore
	if expectedEnergy > MaxEnergy { expectedEnergy = MaxEnergy }
	if h.energy != expectedEnergy {
		t.Fatalf("energy changed unexpectedly: expected %d, got %d", expectedEnergy, h.energy)
	}
	// snapshot should have been appended once
	if h.snaps.len() == 0 {
		t.Fatalf("expected at least one snapshot after action")
	}
}

func TestOldSnapshotsStableAfterBeliefChange(t *testing.T) {
	h := NewHuman("R3")
	HumanInput = seq([]string{"."})
	snap := fakeSnapPos{pos: core.Position{X:0,Y:0}, tick: 30}
	_ = h.Decide(snap)
	if h.snaps.len() == 0 { t.Fatalf("no snapshot recorded") }
	// capture copy of snapshot report
	s0, ok := h.snaps.getFromNewest(0)
	if !ok { t.Fatalf("failed to retrieve newest snapshot") }
	repBefore := s0.Report
	// mutate memory now
	h.memory.tiles[core.Position{X:9,Y:9}] = MemoryTile{Tile: core.TileView{Position: core.Position{X:9,Y:9}}, LastSeen: 31}
	// read snapshot again
	s1, ok := h.snaps.getFromNewest(0)
	if !ok { t.Fatalf("failed to retrieve newest snapshot 2") }
	if s1.Report != repBefore {
		t.Fatalf("snapshot report changed after memory mutation: before=%v after=%v", repBefore, s1.Report)
	}
}

func TestSnapshotRingBounds(t *testing.T) {
	h := NewHuman("R4")
	HumanInput = seq(make([]string, 40)) // produce many empty inputs -> treated as WAIT
	for i := 0; i < 40; i++ {
		_ = h.Decide(fakeSnapPos{pos: core.Position{X:0,Y:0}, tick: i})
	}
	if h.snaps.len() != 32 {
		t.Fatalf("expected ring buffer length 32, got %d", h.snaps.len())
	}
}
