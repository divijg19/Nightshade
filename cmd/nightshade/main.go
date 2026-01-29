package main

import (
	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/runtime"
)

func main() {
	human := agent.NewHuman("You")
	npc := agent.NewOscillating("B")

	rt := runtime.New([]agent.Agent{human, npc})

	for i := 0; i < 300; i++ {
		_ = rt.TickOnce()
	}
}
