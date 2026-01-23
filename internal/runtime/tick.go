package runtime

import (
	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/world"
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
		newPos := applyMovement(pos, action)
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
	switch action {
	case 0: // MOVE_N
		return world.Position{X: pos.X, Y: pos.Y - 1}
	case 1: // MOVE_S
		return world.Position{X: pos.X, Y: pos.Y + 1}
	case 2: // MOVE_E
		return world.Position{X: pos.X + 1, Y: pos.Y}
	case 3: // MOVE_W
		return world.Position{X: pos.X - 1, Y: pos.Y}
	default:
		return pos
	}
}
