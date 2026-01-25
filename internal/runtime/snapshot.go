package runtime

type Position struct {
	X, Y int
}

type TileView struct {
	Position Position
	Glyph    rune
	Visible  bool
}

type Snapshot struct {
	Tick     int
	SelfID   string
	Position Position
	Health   int
	Energy   int
	Visible  []TileView
	Known    []TileView
}

// TickValue returns the current tick for compatibility with agent-side accessors.
func (s Snapshot) TickValue() int { return s.Tick }
