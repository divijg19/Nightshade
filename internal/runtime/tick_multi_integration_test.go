package runtime

import (
	"testing"

	"github.com/divijg19/Nightshade/internal/agent"
)

// Verifies positions over multiple ticks for scripted (east) and oscillating agents.
func TestTickMultipleTicks_Positions(t *testing.T) {
	a1 := agent.NewScripted("A")
	a2 := agent.NewOscillating("B")

	rt := New([]agent.Agent{a1, a2})

	// Expected positions per tick (before advancing tick):
	// Tick 0: A@(0,0), B@(1,0)
	// After TickOnce -> Tick advances to 1 and positions update accordingly.

	expect := []struct {
		tick  int
		aPosX int
		aPosY int
		bPosX int
		bPosY int
	}{
		{0, 0, 0, 1, 0},
		{1, 1, 0, 1, 0},
		{2, 2, 0, 1, 1},
		{3, 3, 0, 1, 0},
		{4, 4, 0, 1, 1},
	}

	for i, e := range expect {
		// Read positions from world
		aPos, ok := rt.world.PositionOf("A")
		if !ok {
			t.Fatalf("tick %d: missing position for A", i)
		}
		bPos, ok := rt.world.PositionOf("B")
		if !ok {
			t.Fatalf("tick %d: missing position for B", i)
		}

		if aPos.X != e.aPosX || aPos.Y != e.aPosY {
			t.Fatalf("tick %d: A pos = %+v, want (%d,%d)", i, aPos, e.aPosX, e.aPosY)
		}
		if bPos.X != e.bPosX || bPos.Y != e.bPosY {
			t.Fatalf("tick %d: B pos = %+v, want (%d,%d)", i, bPos, e.bPosX, e.bPosY)
		}

		// Advance one tick (except after last expectation)
		if i < len(expect)-1 {
			rt.TickOnce()
		}
	}
}
