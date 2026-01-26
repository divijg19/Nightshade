package agent

import "github.com/divijg19/Nightshade/internal/core"

// Memory stores last-known tiles keyed by position. Memory imports only
// internal/core and does not reference runtime.
type Memory struct {
	tiles map[core.Position]core.TileView
}

// NewMemory constructs an empty Memory.
func NewMemory() *Memory {
	return &Memory{tiles: make(map[core.Position]core.TileView)}
}

// UpdateFromVisible accepts an opaque observation and updates memory only
// from what the runtime reports as currently visible. This uses a capability
// interface rather than depending on runtime concrete types.
func (m *Memory) UpdateFromVisible(obs interface{}) {
	if m == nil {
		return
	}
	type visorner interface {
		VisibleTiles() []core.TileView
	}
	if v, ok := obs.(visorner); ok {
		for _, tv := range v.VisibleTiles() {
			m.tiles[tv.Position] = tv
		}
	}
}

// All returns all TileViews currently remembered in memory.
func (m *Memory) All() []core.TileView {
	if m == nil {
		return nil
	}
	out := make([]core.TileView, 0, len(m.tiles))
	for _, tv := range m.tiles {
		out = append(out, tv)
	}
	return out
}

// Count returns the number of known tiles in memory.
func (m *Memory) Count() int {
	if m == nil {
		return 0
	}
	return len(m.tiles)
}

// Get returns the stored TileView and whether it exists.
func (m *Memory) Get(pos core.Position) (core.TileView, bool) {
	if m == nil {
		return core.TileView{}, false
	}
	tv, ok := m.tiles[pos]
	return tv, ok
}
