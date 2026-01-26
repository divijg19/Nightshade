package runtime

import (
	"testing"

	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/world"
)

// Additional edge tests for north and west bounds and a simple multi-agent
// simultaneous movement test.
// simpleAgent is a tiny agent implementation that returns a fixed action.
type simpleAgent struct {
	id  string
	act agent.Action
}

func (a *simpleAgent) ID() string                           { return a.id }
func (a *simpleAgent) Decide(_ agent.Snapshot) agent.Action { return a.act }

func TestTickOnce_NorthWestBoundsAndMultiAgent(t *testing.T) {
	// North bound: agent at Y=0 trying to move north should stay.
	north := &simpleAgent{id: "N", act: agent.MOVE_N}
	rt := New([]agent.Agent{north})
	rt.world.SetPosition("N", world.Position{X: 0, Y: 0})
	beforeN, _ := rt.world.PositionOf("N")
	rt.TickOnce()
	afterN, _ := rt.world.PositionOf("N")
	if beforeN != afterN {
		t.Fatalf("north-bound move occurred: before=%+v after=%+v", beforeN, afterN)
	}

	// West bound: agent at X=0 trying to move west should stay.
	west := &simpleAgent{id: "W", act: agent.MOVE_W}
	rt2 := New([]agent.Agent{west})
	rt2.world.SetPosition("W", world.Position{X: 0, Y: 0})
	beforeW, _ := rt2.world.PositionOf("W")
	rt2.TickOnce()
	afterW, _ := rt2.world.PositionOf("W")
	if beforeW != afterW {
		t.Fatalf("west-bound move occurred: before=%+v after=%+v", beforeW, afterW)
	}

	// Multi-agent simultaneous move: two agents move north from Y=2 to Y=1
	m1 := &simpleAgent{id: "A1", act: agent.MOVE_N}
	m2 := &simpleAgent{id: "A2", act: agent.MOVE_N}
	rt3 := New([]agent.Agent{m1, m2})
	rt3.world.SetPosition("A1", world.Position{X: 0, Y: 2})
	rt3.world.SetPosition("A2", world.Position{X: 1, Y: 2})
	rt3.TickOnce()
	pos1, _ := rt3.world.PositionOf("A1")
	pos2, _ := rt3.world.PositionOf("A2")
	if pos1.Y != 1 || pos2.Y != 1 {
		t.Fatalf("multi-agent north move failed: pos1=%+v pos2=%+v", pos1, pos2)
	}
}

// Snapshot health/energy baseline test: verify fields exist and persist (initially zero).
func TestSnapshot_HealthEnergyBaseline(t *testing.T) {
	a := agent.NewScripted("S")
	rt := New([]agent.Agent{a})
	snap, ok := rt.SnapshotForDebug("S")
	if !ok {
		t.Fatal("expected snapshot for S")
	}
	if snap.Health != 0 || snap.Energy != 0 {
		t.Fatalf("expected initial health/energy 0, got health=%d energy=%d", snap.Health, snap.Energy)
	}
	rt.TickOnce()
	snap2, _ := rt.SnapshotForDebug("S")
	if snap2.Health != 0 || snap2.Energy != 0 {
		t.Fatalf("expected health/energy to persist at 0, got health=%d energy=%d", snap2.Health, snap2.Energy)
	}
}
