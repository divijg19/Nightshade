package runtime

import (
	"testing"

	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/world"
)

type actionAgent struct {
	id  string
	act agent.Action
}

func (a *actionAgent) ID() string                           { return a.id }
func (a *actionAgent) Decide(_ agent.Snapshot) agent.Action { return a.act }

func TestTickOnce_EdgeBoundsPreventMove(t *testing.T) {
	// East bound test: agent at Width-1 trying to move east should stay.
	east := &actionAgent{id: "E", act: agent.MOVE_E}
	rt := New([]agent.Agent{east})
	rt.world.SetPosition("E", world.Position{X: world.Width - 1, Y: 0})

	before, _ := rt.world.PositionOf("E")
	rt.TickOnce()
	after, _ := rt.world.PositionOf("E")
	if before != after {
		t.Fatalf("east-bound move occurred: before=%+v after=%+v", before, after)
	}

	// South bound test: agent at Height-1 trying to move south should stay.
	south := &actionAgent{id: "S", act: agent.MOVE_S}
	rt2 := New([]agent.Agent{south})
	rt2.world.SetPosition("S", world.Position{X: 0, Y: world.Height - 1})
	beforeS, _ := rt2.world.PositionOf("S")
	rt2.TickOnce()
	afterS, _ := rt2.world.PositionOf("S")
	if beforeS != afterS {
		t.Fatalf("south-bound move occurred: before=%+v after=%+v", beforeS, afterS)
	}

	// Within-bounds test: agent near east bound should move if inside.
	mover := &actionAgent{id: "M", act: agent.MOVE_E}
	rt3 := New([]agent.Agent{mover})
	rt3.world.SetPosition("M", world.Position{X: world.Width - 2, Y: 0})
	beforeM, _ := rt3.world.PositionOf("M")
	rt3.TickOnce()
	afterM, _ := rt3.world.PositionOf("M")
	if afterM.X != beforeM.X+1 {
		t.Fatalf("expected M to move east: before=%+v after=%+v", beforeM, afterM)
	}
}
