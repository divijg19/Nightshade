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
	return s.Known
}

// TickValue returns the current tick for compatibility with agent-side accessors.
func (s Snapshot) TickValue() int { return s.Tick }
