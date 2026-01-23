package runtime

type Position struct {
	X, Y int
}

type TileView struct {
	Position Position
	Glyph    rune
}

type Snapshot struct {
	Tick      int
	SelfID    string
	Positions Position
	Health    int
	Energy    int
	Visible   []TileView
}
