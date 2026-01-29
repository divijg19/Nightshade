package agent

import (
	"testing"

	"github.com/divijg19/Nightshade/internal/core"
)

// fake input helper to inject a single line
func makeInput(s string) func() (string, error) {
	return func() (string, error) { return s, nil }
}

// snapWithPos provides VisibleTiles, TickValue and PositionValue for tests
// It relies on the package-level fakeSnap / fakeSnapPos types defined
// in other test files in this package.
type snapWithPos struct{ fakeSnap; pos core.Position }
func (s snapWithPos) PositionValue() core.Position { return s.pos }
func (s snapWithPos) TickValue() int { return s.tick }
func (s snapWithPos) VisibleTiles() []core.TileView { return s.tiles }

func TestHumanInvalidInputWaits(t *testing.T) {
	h := NewHuman("H1")
	HumanInput = makeInput("x")
	defer func(){ HumanInput = nil }()
	snap := fakeSnapPos{pos: core.Position{X:0,Y:0}, tick: 10}
	act := h.Decide(snap)
	if act != WAIT {
		t.Fatalf("expected WAIT for invalid input, got %v", act)
	}
}

func TestHumanObserveClearsScars(t *testing.T) {
	h := NewHuman("H2")
	// scar on a remote memory tile (not currently visible)
	tilePos := core.Position{X: 5, Y: 5}
	h.Memory().tiles[tilePos] = MemoryTile{Tile: core.TileView{Position: tilePos}, LastSeen: 0, ScarLevel: 2}

	// input OBSERVE
	HumanInput = makeInput("e")
	defer func(){ HumanInput = nil }()

	// provide snapshot that does not include tilePos so UpdateFromVisible
	// won't overwrite the scar; Decide should decrement it by 1.
	snap := fakeSnap{tiles: []core.TileView{}, tick: 10}
	_ = h.Decide(snap)
	mt, ok := h.Memory().GetMemoryTile(tilePos)
	if !ok {
		t.Fatalf("expected scarred tile present in memory")
	}
	if mt.ScarLevel != 1 {
		t.Fatalf("expected scar level 1 after OBSERVE, got %d", mt.ScarLevel)
	}
}

func TestHumanHallucinatesUnderParanoia(t *testing.T) {
	h := NewHuman("H3")
	mem := h.Memory()
	tick := 100
	target := core.Position{X: 2, Y: 2}
	mem.tiles[target] = MemoryTile{Tile: core.TileView{Position: target, Glyph: 'Z'}, LastSeen: tick - (ParanoiaThreshold + 1)}

	// use buildObservation directly to inspect hallucination
	snap := fakeSnap{tiles: []core.TileView{}, tick: tick}
	prev := make(map[core.Position]MemoryTile)
	obs := buildObservation(mem, snap, prev, MaxEnergy, ParanoiaThreshold)
	found := false
	for _, v := range obs.Visible { if v.Position == target { found = true; break } }
	if !found { t.Fatalf("expected hallucination present in Visible") }
}

func TestHumanAffectedByBeliefContagion(t *testing.T) {
	// Set up an emitter signal for this tick
	tick := 50
	senderPos := core.Position{X:0, Y:0}
	tilePos := core.Position{X:1, Y:0}
	emitBeliefSignal("S", tick, senderPos, []Belief{{Tile: core.TileView{Position: tilePos}, Age: 0}})

	// human positioned at (0,0) should receive contagion when Decide runs
	h := NewHuman("H4")
	HumanInput = makeInput(".")
	defer func(){ HumanInput = nil }()
	snap := fakeSnap{tiles: []core.TileView{}, tick: tick}
	// snapshot position must be near sender; use package-level snapWithPos
	s := snapWithPos{fakeSnap: snap, pos: core.Position{X:0,Y:0}}
	_ = h.Decide(s)
	// after Decide, human memory should contain tilePos (transferred)
	if _, ok := h.Memory().GetMemoryTile(tilePos); !ok {
		t.Fatalf("expected contagion to transfer belief into human memory")
	}
}
