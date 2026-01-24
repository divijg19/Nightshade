package runtime

import (
	"testing"

	"github.com/divijg19/Nightshade/internal/agent"
)

type westAgent struct{ id string }

func (w *westAgent) ID() string { return w.id }
func (w *westAgent) Decide(snapshot agent.Snapshot) agent.Action { return agent.MOVE_W }

func TestTickOnce_PreventOutOfBoundsWest(t *testing.T) {
    a := &westAgent{id: "W"}
    rt := New([]agent.Agent{a})

    // runtime.New initializes positions to X=index (0) Y=0. At X=0, moving west
    // should be prevented by ResolveMovement.
    before, ok := rt.world.PositionOf("W")
    if !ok {
        t.Fatal("expected agent position to be set")
    }
    if before.X != 0 {
        t.Fatalf("unexpected initial X: %d", before.X)
    }

    rt.TickOnce()

    after, ok := rt.world.PositionOf("W")
    if !ok {
        t.Fatal("expected agent position after tick")
    }
    if after != before {
        t.Fatalf("agent moved out of bounds: before=%+v after=%+v", before, after)
    }
}
