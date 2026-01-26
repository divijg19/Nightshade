package main

import (
	"fmt"

	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/core"
	"github.com/divijg19/Nightshade/internal/runtime"
)

func main() {
	var a1 agent.Agent = agent.NewScripted("A")
	var a2 agent.Agent = agent.NewOscillating("B")

	rtAgents := []agent.Agent{a1, a2}
	rt := runtime.New(rtAgents)

	for i := 0; i < 60; i++ {
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
			// Debug: 'sees' is current visibility, 'believes' is agent memory
			fmt.Printf("Agent A sees %d tiles: %+v\n", len(snap.Visible), snap.Visible)
			if s, ok := a1.(*agent.Scripted); ok {
				fmt.Printf("Agent A sees %d tiles, believes %d tiles\n", len(snap.Visible), s.Memory().Count())

				// Debug: print authoritative marker position and agent belief
				truth := rt.MarkerPosition()
				believed := []core.Position{}
				for _, tv := range s.Memory().All() {
					if tv.Glyph == 'M' {
						believed = append(believed, tv.Position)
					}
				}
				fmt.Printf("Truth marker at %v, agent believes marker at %v\n", truth, believed)
				if len(believed) == 1 {
					bp := believed[0]
					if bp.X != truth.X || bp.Y != truth.Y {
						fmt.Printf("FALSE BELIEF: agent believes marker at %v but truth is %v\n", bp, truth)
					}
				}
			} else {
				fmt.Printf("Agent A sees %d tiles\n", len(snap.Visible))
			}
		}
		if s, ok := a1.(*agent.Scripted); ok {
			fmt.Printf("Agent A remembers %d tiles\n", s.Memory().Count())
		}
	}
}
