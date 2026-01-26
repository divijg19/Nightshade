package main

import (
	"fmt"

	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/runtime"
)

func main() {
	var a1 agent.Agent = agent.NewScripted("A")
	var a2 agent.Agent = agent.NewOscillating("B")

	rtAgents := []agent.Agent{a1, a2}
	rt := runtime.New(rtAgents)

	for i := 0; i < 5; i++ {
		decisions := rt.TickOnce()
		fmt.Printf("Tick %d\n", rt.Tick())
		for id, action := range decisions {
			fmt.Printf("  Agent %s -> Action %d\n", id, action)
		}
		snapA, ok := rt.SnapshotForDebug("A")
		if ok {
			fmt.Printf("  Agent A position: %+v\n", snapA.Position)
		}
		if snap, ok := rt.SnapshotForDebug("A"); ok {
			fmt.Printf("Agent A sees %d tiles: %+v\n", len(snap.Visible), snap.Visible)
			fmt.Printf(
				"Agent A sees %d tiles, knows %d tiles\n",
				len(snap.Visible),
				len(snap.Known),
			)
		}
		if s, ok := a1.(*agent.Scripted); ok {
			fmt.Printf("Agent A remembers %d tiles\n", s.Memory().Count())
		}
	}
}
