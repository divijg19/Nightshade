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
	// Use the package bounds constants to construct the world so tests
	// that reference `world.Width`/`world.Height` match runtime size.
	w := world.New(world.Width, world.Height)

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
