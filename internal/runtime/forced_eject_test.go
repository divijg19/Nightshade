package runtime

import (
	"testing"

	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/core"
)

// Test that forced eject occurs exactly when Pressure reaches MaxPressure
// and that the signal is burned, the dungeon removed, and a scar applied.
func TestForcedEject_AtThreshold(t *testing.T) {
    a := agent.NewRemoteHumanFromExisting("A", agent.NewMemory(), agent.MaxEnergy)
    rt := New([]agent.Agent{a})

    // Enter S000
    a.RecvInput <- "ENTER_SIGNAL S000"
    rt.TickOnce()
    _ = mustRecvObs(t, a)

    d := rt.dungeonByAgent["A"]
    if d == nil {
        t.Fatalf("expected dungeon after enter")
    }
    max := d.MaxPressure

    // Tick until threshold. On the tick when pressure reaches MaxPressure,
    // the agent should be ejected and see a board-mode snapshot.
    for i := 1; i <= max; i++ {
        a.RecvInput <- "."
        rt.TickOnce()
        obs := mustRecvObs(t, a)
        if i < max {
            if obs.Mode != "dungeon" {
                t.Fatalf("expected dungeon mode before eject (i=%d), got %q", i, obs.Mode)
            }
        } else {
            if obs.Mode != "board" {
                t.Fatalf("expected board mode on eject tick, got %q", obs.Mode)
            }
            // runtime should no longer have the dungeon binding
            if _, ok := rt.dungeonByAgent["A"]; ok {
                t.Fatalf("expected dungeon binding removed after eject")
            }
            // signal should be burned and unlocked
            s, ok := rt.board.Find("S000")
            if !ok {
                t.Fatalf("expected signal S000 to exist")
            }
            if !s.Burned {
                t.Fatalf("expected S000 to be burned on eject")
            }
            if s.LockedBy != "" {
                t.Fatalf("expected S000 to be unlocked after burn, lockedBy=%q", s.LockedBy)
            }
            // memory scar applied at agent position
            if mt, found := a.Memory().GetMemoryTile(core.Position{X: 0, Y: 0}); !found || mt.ScarLevel < 1 {
                t.Fatalf("expected scar applied to agent memory at pos (0,0), got found=%v scar=%d", found, mt.ScarLevel)
            }
        }
    }
}

// Test forced eject isolation: ejecting one agent must not affect another agent
func TestForcedEject_Isolation(t *testing.T) {
    a := agent.NewRemoteHumanFromExisting("A", agent.NewMemory(), agent.MaxEnergy)
    b := agent.NewRemoteHumanFromExisting("B", agent.NewMemory(), agent.MaxEnergy)
    rt := New([]agent.Agent{a, b})

    // Tick 0: A enters S000, B waits on board
    a.RecvInput <- "ENTER_SIGNAL S000"
    b.RecvInput <- "."
    rt.TickOnce()
    _ = mustRecvObs(t, a)
    _ = mustRecvObs(t, b)

    d := rt.dungeonByAgent["A"]
    if d == nil {
        t.Fatalf("expected dungeon for A")
    }
    max := d.MaxPressure

    // Advance ticks until A ejects; B continues to send idle inputs and must remain unaffected.
    for i := 1; i <= max; i++ {
        a.RecvInput <- "."
        b.RecvInput <- "."
        rt.TickOnce()
        obsA := mustRecvObs(t, a)
        obsB := mustRecvObs(t, b)
        if i < max {
            if obsA.Mode != "dungeon" {
                t.Fatalf("expected A in dungeon before eject (i=%d), got %q", i, obsA.Mode)
            }
            if obsB.Mode != "board" {
                t.Fatalf("expected B on board before eject, got %q", obsB.Mode)
            }
        } else {
            // eject tick: A should see board, B should remain board
            if obsA.Mode != "board" {
                t.Fatalf("expected A ejected on final tick, got %q", obsA.Mode)
            }
            if obsB.Mode != "board" {
                t.Fatalf("expected B to remain on board after A eject, got %q", obsB.Mode)
            }
            // Ensure B was not bound to any dungeon
            if _, ok := rt.dungeonByAgent["B"]; ok {
                t.Fatalf("expected B not to be bound to a dungeon")
            }
        }
    }
}

// Test that the one-shot pending event set during forced eject is delivered
// on the subsequent snapshot (board mode) when the agent is ejected.
func TestForcedEject_DeliversEventOnEject(t *testing.T) {
    a := agent.NewRemoteHumanFromExisting("A", agent.NewMemory(), agent.MaxEnergy)
    rt := New([]agent.Agent{a})

    // Enter S000
    a.RecvInput <- "ENTER_SIGNAL S000"
    rt.TickOnce()
    _ = mustRecvObs(t, a)

    d := rt.dungeonByAgent["A"]
    if d == nil {
        t.Fatalf("expected dungeon after enter")
    }
    max := d.MaxPressure

    // Advance to eject threshold
    for i := 1; i <= max; i++ {
        a.RecvInput <- "."
        rt.TickOnce()
        obs := mustRecvObs(t, a)
        if i == max {
            if obs.Mode != "board" {
                t.Fatalf("expected board mode on eject tick, got %q", obs.Mode)
            }
            if obs.Event == "" {
                t.Fatalf("expected one-shot eject event delivered on board snapshot")
            }
        }
    }
}
