package common

import (
	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/core"
	"github.com/divijg19/Nightshade/internal/runtime"
)

func BootstrapObservation(rt *runtime.Runtime, agentID string) agent.Observation {
	var snap runtime.Snapshot
	var hasSnap bool
	if rt != nil {
		if s, ok := rt.SnapshotForDebug(agentID); ok {
			snap = s
			hasSnap = true
		}
	}
	if !hasSnap {
		snap = runtime.Snapshot{Tick: 0, Position: core.Position{X: 0, Y: 0}, Visible: nil}
	}
	obs := agent.Observation{
		Visible:  snap.Visible,
		Known:    []agent.Belief{},
		Tick:     snap.Tick,
		Position: snap.Position,
		Presence: nil,
	}
	obs.Mode = snap.Mode
	if len(snap.Board.Signals) > 0 {
		b := snap.Board
		obs.Board = &b
	}
	if snap.Dungeon.ExitStability > 0 || snap.Dungeon.Pressure > 0 || snap.Dungeon.AnchorType != "" {
		d := snap.Dungeon
		obs.Dungeon = &d
		if obs.Mode == "" {
			obs.Mode = "dungeon"
		}
	}
	return obs
}
