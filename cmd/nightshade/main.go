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

	for i := 0; i < 300; i++ {
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
				// Also print belief ages for marker entries
				for _, mt := range s.Memory().All() {
					if mt.Tile.Glyph == 'M' {
						believed = append(believed, mt.Tile.Position)
						age := snap.Tick - mt.LastSeen
						fmt.Printf("Agent believes marker at %v, last seen %d ticks ago\n", mt.Tile.Position, age)
					}
				}
				fmt.Printf("Truth marker at %v, agent believes marker at %v\n", truth, believed)
				if len(believed) == 1 {
					bp := believed[0]
					if bp.X != truth.X || bp.Y != truth.Y {
						fmt.Printf("FALSE BELIEF: agent believes marker at %v but truth is %v\n", bp, truth)
					}
				}

				// Reconstruct intended action (Scripted always intends MOVE_E).
				// Compute target from snapshot Position.
				target := core.Position{X: snap.Position.X + 1, Y: snap.Position.Y}
				if mt, ok := s.Memory().GetMemoryTile(target); ok {
					age := snap.Tick - mt.LastSeen
					if age > agent.CautionThreshold && decisions["A"] == agent.OBSERVE {
						fmt.Printf("Agent A intends MOVE_E -> target age=%d -> OBSERVE (caution)\n", age)
					} else {
						fmt.Printf("Agent A moves MOVE_E (target age=%d)\n", age)
					}
				} else {
					// Tile never seen -> moves
					fmt.Printf("Agent A intends MOVE_E -> target unseen -> moves\n")
				}

			// If the agent chose to OBSERVE, print detailed refresh debug lines.
			if decisions["A"] == agent.OBSERVE {
				visCount := len(snap.Visible)
				fmt.Printf("Agent A OBSERVES -> refreshing belief for %d visible tiles\n", visCount)
				for _, vtv := range snap.Visible {
					if mt, ok := s.Memory().GetMemoryTile(vtv.Position); ok {
						age := snap.Tick - mt.LastSeen
						fmt.Printf("Tile (%d,%d) belief age reset to %d\n", vtv.Position.X, vtv.Position.Y, age)
					}
				}
			}

				// Debug: show age for a remembered tile (if any) to demonstrate age growth
				if s.Memory().Count() > 0 {
					mts := s.Memory().All()
					mt := mts[0]
					age := snap.Tick - mt.LastSeen
					// Check whether the tile is currently visible (age should be 0)
					visibleNow := false
					for _, vtv := range snap.Visible {
						if vtv.Position == mt.Tile.Position {
							visibleNow = true
							break
						}
					}
					if visibleNow {
						fmt.Printf("Agent just observed tile at %v, age reset to %d\n", mt.Tile.Position, age)
					} else {
						fmt.Printf("Agent remembers tile at %v, last seen %d ticks ago\n", mt.Tile.Position, age)
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
