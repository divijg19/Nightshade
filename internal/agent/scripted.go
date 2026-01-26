package agent

import "github.com/divijg19/Nightshade/internal/core"

type Scripted struct {
	id     string
	memory *Memory
}

func NewScripted(id string) *Scripted {
	return &Scripted{id: id,
		memory: NewMemory(),
	}
}

func (s *Scripted) ID() string {
	return s.id
}

func (s *Scripted) Decide(snapshot Snapshot) Action {
	// Update memory only from what is currently visible.
	if s.memory != nil {
		s.memory.UpdateFromVisible(snapshot)
	}

	// Build an agent-side Observation (belief = memory.All()). Agents may
	// start to act on Observation in later steps; for now we ignore it.
	var vis []core.TileView
	if v, ok := snapshot.(interface{ VisibleTiles() []core.TileView }); ok {
		vis = v.VisibleTiles()
	}
	tick := 0
	if t, ok := snapshot.(interface{ TickValue() int }); ok {
		tick = t.TickValue()
	}
	// Build Known beliefs from memory: compute Age = obs.Tick - LastSeen
	known := []Belief{}
	for _, mt := range s.memory.All() {
		age := tick - mt.LastSeen
		known = append(known, Belief{Tile: mt.Tile, Age: age})
	}
	obs := Observation{
		Visible: vis,
		Known:   known,
		Tick:    tick,
	}
	_ = obs
	return MOVE_E
}

// Memory exposes the agent's memory for external inspection in tools/tests.
func (s *Scripted) Memory() *Memory {
	return s.memory
}

// Oscillating moves north on even ticks and south on odd ticks.
type Oscillating struct {
	id     string
	memory *Memory
}

func NewOscillating(id string) *Oscillating {
	return &Oscillating{id: id,
		memory: NewMemory(),
	}
}

func (o *Oscillating) ID() string {
	return o.id
}

func (o *Oscillating) Decide(snapshot Snapshot) Action {
	// Update memory from visible tiles only.
	if o.memory != nil {
		o.memory.UpdateFromVisible(snapshot)
	}

	// Build agent-side Observation for future use; currently ignored.
	var vis []core.TileView
	if v, ok := snapshot.(interface{ VisibleTiles() []core.TileView }); ok {
		vis = v.VisibleTiles()
	}
	tick := 0
	if t, ok := snapshot.(interface{ TickValue() int }); ok {
		tick = t.TickValue()
	}
	known := []Belief{}
	for _, mt := range o.memory.All() {
		age := tick - mt.LastSeen
		known = append(known, Belief{Tile: mt.Tile, Age: age})
	}
	obs := Observation{
		Visible: vis,
		Known:   known,
		Tick:    tick,
	}
	_ = obs

	// Decide using tick parity as before.
	if t, ok := snapshot.(interface{ TickValue() int }); ok {
		if t.TickValue()%2 == 0 {
			return MOVE_N
		}
		return MOVE_S
	}

	// Fallback: treat as even tick
	return MOVE_N
}

// Memory exposes the agent's memory for external inspection in tools/tests.
func (o *Oscillating) Memory() *Memory {
	return o.memory
}
