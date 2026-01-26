package agent

import "github.com/divijg19/Nightshade/internal/core"

// Belief pairs a remembered tile with its age (in ticks since last seen).
type Belief struct {
	Tile core.TileView
	Age  int
}

// Observation is the agent-side interpretation of a runtime Snapshot.
// It separates ephemeral Visible tiles from the agent's persistent Known
// belief (built from Memory). Known contains Belief entries computed from
// Memory.LastSeen and the current Tick.
type Observation struct {
	Visible []core.TileView
	Known   []Belief
	Tick    int
}
