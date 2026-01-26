package runtime

import "github.com/divijg19/Nightshade/internal/core"

type Snapshot struct {
	Tick     int
	SelfID   string
	Position core.Position
	Health   int
	Energy   int
	Visible  []core.TileView
	Known    []core.TileView
}

func (s Snapshot) KnownTiles() []core.TileView {
	// Keep this for backward compatibility: Runtime no longer fabricates
	// Known; agents should treat Snapshot.Known as empty and rely on their
	// Memory for persistent belief. This method returns the raw Known slice
	// (typically empty).
	return s.Known
}

// VisibleTiles returns the tiles seen in the current tick.
func (s Snapshot) VisibleTiles() []core.TileView { return s.Visible }

// TickValue returns the current tick for compatibility with agent-side accessors.
func (s Snapshot) TickValue() int { return s.Tick }
