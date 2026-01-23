package world

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
