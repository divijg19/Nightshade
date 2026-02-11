package runtime

import (
	"testing"

	"github.com/divijg19/Nightshade/internal/agent"
)

// Board should expose burned signals in the BoardView so the client can render them.
func TestBoard_ShowsBurnedSignals(t *testing.T) {
	a := agent.NewRemoteHumanFromExisting("A", agent.NewMemory(), agent.MaxEnergy)
	rt := New([]agent.Agent{a})

	// Ensure S000 exists initially
	if s, ok := rt.board.Find("S000"); !ok {
		t.Fatalf("expected S000 to exist initially, got ok=%v", ok)
	} else {
		if s.Burned {
			t.Fatalf("expected S000 not burned initially")
		}
	}

	// Burn S000 and tick to allow snapshot propagation
	rt.board.Burn("S000")
	a.RecvInput <- "."
	rt.TickOnce()
	obs := mustRecvObs(t, a)

	if obs.Mode != "board" {
		t.Fatalf("expected board mode, got %q", obs.Mode)
	}
	if obs.Board == nil {
		t.Fatalf("expected board view in observation")
	}
	found := false
	for _, sv := range obs.Board.Signals {
		if sv.ID == "S000" {
			found = true
			if !sv.Burned {
				t.Fatalf("expected S000 to be marked burned in snapshot")
			}
		}
	}
	if !found {
		t.Fatalf("expected S000 present in board snapshot signals")
	}
}
