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

// UpdateFromSnapshot accepts an opaque observation and updates memory if the
// observation exposes KnownTiles() []core.TileView. This uses a capability
// interface rather than depending on runtime concrete types.
func (m *Memory) UpdateFromSnapshot(obs interface{}) {
	if m == nil {
		return
	}
	type knowner interface {
		KnownTiles() []core.TileView
	}
	if k, ok := obs.(knowner); ok {
		for _, tv := range k.KnownTiles() {
			m.tiles[tv.Position] = tv
		}
	}
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
