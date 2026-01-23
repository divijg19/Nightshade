package runtime

import "github.com/divijg19/Nightshade/internal/agent"

type Runtime struct {
	tick   int
	agents []agent.Agent
}

func New(agents []agent.Agent) *Runtime {
	return &Runtime{
		tick:   0,
		agents: agents,
	}
}

func (r *Runtime) Tick() int {
	return r.tick
}

func (r *Runtime) advanceTick() {
	r.tick++
}
