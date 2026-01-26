package world

// World bounds. Keep these as simple constants for now; movement logic will
// consult these to prevent out-of-bounds moves.
const (
	Width  = 80
	Height = 25
)

type Position struct {
	X, Y int
}

type World struct {
	width    int
	height   int
	entities map[string]Position
	marker   Marker
}

func New(width, height int) *World {
	return &World{
		width:    width,
		height:   height,
		entities: make(map[string]Position),
		marker: Marker{
			Position: Position{
				X: width / 2,
				Y: height / 2,
			},
		},
	}
}

// Marker is a simple world fact that moves deterministically each tick.
type Marker struct {
	Position Position
}

// MoveMarker advances the marker one cell to the east, wrapping at world edge.
func (w *World) MoveMarker() {
	w.marker.Position.X++
	if w.marker.Position.X >= w.width {
		w.marker.Position.X = 0
	}
}

// MarkerPosition returns the current marker position.
func (w *World) MarkerPosition() Position {
	return w.marker.Position
}

func (w *World) Width() int {
	return w.width
}

func (w *World) Height() int {
	return w.height
}

func (w *World) PositionOf(id string) (Position, bool) {
	pos, ok := w.entities[id]
	return pos, ok
}

func (w *World) SetPosition(id string, pos Position) {
	w.entities[id] = pos
}
