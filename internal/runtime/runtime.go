package runtime

import (
	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/world"
)

const defaultVisibilityRadius = 2

type Runtime struct {
	tick   int
	agents []agent.Agent
	world  *world.World
}

func New(agents []agent.Agent) *Runtime {
	w := world.New(10, 10) // Initial dimeonsions of the world, will change to 20x20

	for i, a := range agents {
		w.SetPosition(a.ID(), world.Position{
			X: i,
			Y: 0,
		})
	}
	return &Runtime{
		tick:   0,
		agents: agents,
		world:  w,
	}
}

func (r *Runtime) Tick() int {
	return r.tick
}

func (r *Runtime) advanceTick() {
	r.tick++
}

func (r *Runtime) SnapshotForDebug(agentID string) (Snapshot, bool) {
	for _, a := range r.agents {
		if a.ID() == agentID {
			return r.snapshotFor(a, agent.Action(-1)), true
		}
	}
	return Snapshot{}, false
}
