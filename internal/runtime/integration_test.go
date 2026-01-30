package runtime

import (
	"testing"

	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/core"
)

// TestTwoHumansObservationAndMove verifies that two RemoteHuman agents
// receive observations in the observation phase and that a provided input
// moves the corresponding agent during resolution.
func TestTwoHumansObservationAndMove(t *testing.T) {
    a := agent.NewRemoteHumanFromExisting("A", agent.NewMemory(), agent.MaxEnergy)
    b := agent.NewRemoteHumanFromExisting("B", agent.NewMemory(), agent.MaxEnergy)

    rt := New([]agent.Agent{a, b})

    // Provide input for agent A to move east ('d') and none for B.
    a.RecvInput <- "d"

    // Run one tick.
    _ = rt.TickOnce()
    // Run a second tick to ensure contagion has opportunity across phases.
    _ = rt.TickOnce()

    // Both agents should have received an observation for tick 0.
    select {
    case obs := <-a.SendObservation:
        if obs.Tick != 0 {
            t.Fatalf("unexpected tick for A obs: %d", obs.Tick)
        }
    default:
        t.Fatalf("agent A did not receive observation")
    }
    select {
    case obs := <-b.SendObservation:
        if obs.Tick != 0 {
            t.Fatalf("unexpected tick for B obs: %d", obs.Tick)
        }
    default:
        t.Fatalf("agent B did not receive observation")
    }

    // After resolution, A should have moved from (0,0) to (1,0).
    if pos, ok := rt.world.PositionOf("A"); !ok {
        t.Fatalf("missing position for A")
    } else if pos.X != 1 || pos.Y != 0 {
        t.Fatalf("A position = %+v, want (1,0)", pos)
    }
}

// TestContagionTransfer ensures that belief contagion transfers beliefs
// from one agent to a nearby agent during the decision pass.
func TestContagionTransfer(t *testing.T) {
    aMem := agent.NewMemory()
    bMem := agent.NewMemory()

    // Agent A remembers a tile at (0,3) which is outside both agents' visibility
    // radius so it must be transferred via contagion rather than observed.
    tilePos := core.Position{X: 0, Y: 3}
    aMem.SetMemoryTile(tilePos, agent.MemoryTile{Tile: core.TileView{Position: tilePos, Glyph: rune('Z'), Visible: true}, LastSeen: 0})

    a := agent.NewRemoteHumanFromExisting("A", aMem, agent.MaxEnergy)
    b := agent.NewRemoteHumanFromExisting("B", bMem, agent.MaxEnergy)

    rt := New([]agent.Agent{a, b})

    // Sanity: A should have the belief before the tick; B should not.
    if _, ok := a.Memory().GetMemoryTile(tilePos); !ok {
        t.Fatalf("setup failed: A missing initial belief at %v", tilePos)
    }
    if _, ok := b.Memory().GetMemoryTile(tilePos); ok {
        t.Fatalf("setup failed: B unexpectedly has belief at %v", tilePos)
    }

    // No explicit inputs; tick will proceed with empty inputs.
    _ = rt.TickOnce()
        // Inspect emitted belief signals for debugging
        sigs := agent.GetBeliefSignals()
        t.Logf("belief signals: %+v", sigs)

    // Debug: dump A and B memories
    t.Logf("A mem count=%d", a.Memory().Count())
    for _, mt := range a.Memory().All() {
        t.Logf("A mem tile %+v lastSeen=%d", mt.Tile.Position, mt.LastSeen)
    }
    t.Logf("B mem count=%d", b.Memory().Count())
    for _, mt := range b.Memory().All() {
        t.Logf("B mem tile %+v lastSeen=%d", mt.Tile.Position, mt.LastSeen)
    }

    // After tick, B's memory should contain the tile (transferred via contagion)
    if mt, ok := b.Memory().GetMemoryTile(tilePos); !ok {
        t.Fatalf("B did not acquire belief at %v; A mem count=%d B mem count=%d", tilePos, a.Memory().Count(), b.Memory().Count())
    } else {
        // Expect LastSeen to be tick - TransferPenalty (tick is 0 at decision)
        if mt.LastSeen != 0-agent.TransferPenalty {
            t.Fatalf("unexpected LastSeen for transferred belief: %d", mt.LastSeen)
        }
        if mt.Tile.Position != tilePos {
            t.Fatalf("transferred tile position mismatch: %+v", mt.Tile.Position)
        }
    }
}
