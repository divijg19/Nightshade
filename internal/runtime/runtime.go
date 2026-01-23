package runtime

import (
	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/world"
)

type Runtime struct {
	tick   int
	agents []agent.Agent
	world  *world.World
}

func New(agents []agent.Agent) *Runtime {
	return &Runtime{
		tick:   0,
		agents: agents,
		world:  world.New(),
	}
}

func (r *Runtime) Tick() int {
	return r.tick
}

func (r *Runtime) advanceTick() {
	r.tick++
}
