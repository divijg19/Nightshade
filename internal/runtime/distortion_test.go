package runtime

import (
	"testing"

	"github.com/divijg19/Nightshade/internal/agent"
)

// Distortion rotates movement clockwise when pressure>=11 and tick%3==0.
func TestDistortion_RotatesMovement(t *testing.T) {
    a := agent.NewRemoteHumanFromExisting("A", agent.NewMemory(), agent.MaxEnergy)
    rt := New([]agent.Agent{a})

    // Enter dungeon
    a.RecvInput <- "ENTER_SIGNAL S000"
    rt.TickOnce()
    _ = mustRecvObs(t, a)

    // Force conditions: ensure dungeon exists and set pressure>=11 and tick aligned
    a.RecvInput <- "ENTER_SIGNAL S000"
    rt.TickOnce()
    _ = mustRecvObs(t, a)

    d := rt.dungeonByAgent["A"]
    if d == nil {
        t.Fatalf("expected dungeon for A")
    }
    d.Pressure = 11
    // Align runtime tick to a value divisible by 3 to force distortion
    rt.tick = 3

    // When distortion active (rt.tick%3==0 and pressure>=11), a MOVE_N should rotate to MOVE_E
    a.RecvInput <- "w"
    rt.TickOnce()
    _ = mustRecvObs(t, a)
    pos, _ := rt.world.PositionOf("A")
    if pos.X <= 0 {
        t.Fatalf("expected agent to have moved east due to distortion, got pos=%v", pos)
    }
}
