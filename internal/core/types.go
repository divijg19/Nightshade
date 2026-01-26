package core

type Position struct {
	X, Y int
}

type TileView struct {
	Position Position
	Glyph    rune
	Visible  bool
}
