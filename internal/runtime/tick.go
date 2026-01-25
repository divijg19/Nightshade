package runtime

import (
	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/game"
)

type Decisions map[string]agent.Action

func (r *Runtime) TickOnce() Decisions {
	decisions := make(Decisions)

	for _, a := range r.agents {
		snapshot := r.snapshotFor(a)
		action := a.Decide(snapshot)
		decisions[a.ID()] = action

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
