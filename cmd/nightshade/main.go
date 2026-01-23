package main

import (
	"fmt"

	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/runtime"
)

func main() {
	a1 := agent.NewScripted("A")
	a2 := agent.NewScripted("B")

	rt := runtime.New([]agent.Agent{a1, a2})

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
	}
}
