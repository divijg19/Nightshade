package agent

import "github.com/divijg19/Nightshade/internal/core"

// MemoryTile represents a remembered tile and the tick when it was last
// observed. This type lives in the agent layer and imports only internal/core.
type MemoryTile struct {
	Tile     core.TileView
	LastSeen int
}

// Memory stores last-known tiles keyed by position. Memory imports only
// internal/core and does not reference runtime.
type Memory struct {
	tiles map[core.Position]MemoryTile
}

// NewMemory constructs an empty Memory.
func NewMemory() *Memory {
	return &Memory{tiles: make(map[core.Position]MemoryTile)}
}

// UpdateFromVisible accepts an opaque observation and updates memory only
// from what the runtime reports as currently visible. This uses a capability
// interface rather than depending on runtime concrete types. When a tile is
// visible we overwrite the stored Tile and set LastSeen = snapshot.TickValue().
func (m *Memory) UpdateFromVisible(obs interface{}) {
	if m == nil {
		return
	}
	type visTicker interface {
		VisibleTiles() []core.TileView
		TickValue() int
	}
	if v, ok := obs.(visTicker); ok {
		tick := v.TickValue()
		for _, tv := range v.VisibleTiles() {
			m.tiles[tv.Position] = MemoryTile{Tile: tv, LastSeen: tick}
		}
	}
}

// All returns all MemoryTile entries currently remembered in memory.
func (m *Memory) All() []MemoryTile {
	if m == nil {
		return nil
	}
	out := make([]MemoryTile, 0, len(m.tiles))
	for _, mt := range m.tiles {
		out = append(out, mt)
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
// The memory backing stores MemoryTile; this helper returns the TileView.
func (m *Memory) Get(pos core.Position) (core.TileView, bool) {
	if m == nil {
		return core.TileView{}, false
	}
	mt, ok := m.tiles[pos]
	if !ok {
		return core.TileView{}, false
	}
	return mt.Tile, true
}

// GetMemoryTile returns the MemoryTile for a position and whether it exists.
func (m *Memory) GetMemoryTile(pos core.Position) (MemoryTile, bool) {
	if m == nil {
		return MemoryTile{}, false
	}
	mt, ok := m.tiles[pos]
	return mt, ok
}
