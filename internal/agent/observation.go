package agent

import "github.com/divijg19/Nightshade/internal/core"

// Observation is the agent-side interpretation of a runtime Snapshot.
// It separates ephemeral Visible tiles from the agent's persistent Known
// belief (Memory.All()).
type Observation struct {
	Visible []core.TileView
	Known   []core.TileView
	Tick    int
}
