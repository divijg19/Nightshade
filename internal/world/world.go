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
	entities map[string]Position
}

func New() *World {
	return &World{
		entities: make(map[string]Position),
	}
}

func (w *World) PositionOf(id string) (Position, bool) {
	pos, ok := w.entities[id]
	return pos, ok
}

func (w *World) SetPosition(id string, pos Position) {
	w.entities[id] = pos
}
