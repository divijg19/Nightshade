package runtime

import (
	"fmt"

	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/game"
	"github.com/divijg19/Nightshade/internal/world"
)

type Decisions map[string]agent.Action

func (r *Runtime) TickOnce() Decisions {
	decisions := make(Decisions)
	for _, a := range r.agents {
		snapshot := r.snapshotFor(a)
		action := a.Decide(snapshot)
		decisions[a.ID()] = action

		// Print debug snapshot for this agent (use public SnapshotForDebug accessor)
		if dbgSnap, ok := r.SnapshotForDebug(a.ID()); ok {
			printDebugSnapshot(dbgSnap)
		}

		pos, ok := r.world.PositionOf(a.ID())
		if !ok {
			continue
		}
		newPos := game.ResolveMovement(pos, action)
		r.world.SetPosition(a.ID(), newPos)
	}
	r.advanceTick()
	return decisions
}

func (r *Runtime) snapshotFor(a agent.Agent) Snapshot {
	snap := Snapshot{
		Tick:   r.tick,
		SelfID: a.ID(),
	}

	if pos, ok := r.world.PositionOf(a.ID()); ok {
		snap.Position = Position{
			X: pos.X,
			Y: pos.Y,
		}
	}

	return snap
}

func applyMovement(pos world.Position, action agent.Action) world.Position {
	// removed: movement now resolved in internal/game.ResolveMovement
	return pos
}

func printDebugSnapshot(s Snapshot) {
	fmt.Printf("Tick %d Agent %s Position=(%d,%d) Health=%d Energy=%d\n", s.Tick, s.SelfID, s.Position.X, s.Position.Y, s.Health, s.Energy)
}
