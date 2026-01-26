package agent

import "github.com/divijg19/Nightshade/internal/core"

// computeTarget returns the target position for a MOVE action relative to
// the current position. The second return value is false when the action is
// not a movement action.
func computeTarget(pos core.Position, a Action) (core.Position, bool) {
	switch a {
	case MOVE_N:
		return core.Position{X: pos.X, Y: pos.Y - 1}, true
	case MOVE_S:
		return core.Position{X: pos.X, Y: pos.Y + 1}, true
	case MOVE_E:
		return core.Position{X: pos.X + 1, Y: pos.Y}, true
	case MOVE_W:
		return core.Position{X: pos.X - 1, Y: pos.Y}, true
	default:
		return core.Position{}, false
	}
}

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

	// Decision flow: compute intended action (existing behavior), then
	// potentially override with OBSERVE if target belief is stale.
	intended := MOVE_E

	// If intended is a move, compute target from agent's current position.
	if posv, ok := snapshot.(interface{ PositionValue() core.Position }); ok {
		if tgt, ok2 := computeTarget(posv.PositionValue(), intended); ok2 {
			if mt, found := s.memory.GetMemoryTile(tgt); found {
				age := tick - mt.LastSeen
				if age > CautionThreshold {
					// Prefer to OBSERVE (refresh visible belief) instead of a blind WAIT.
					// OBSERVE consumes one tick and refreshes memory from Visible (already done above).
					return OBSERVE
				}
			}
		}
	}

	_ = obs
	return intended
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

	// Decide using tick parity as before, then apply caution check.
	intended := MOVE_N
	if t, ok := snapshot.(interface{ TickValue() int }); ok {
		if t.TickValue()%2 == 0 {
			intended = MOVE_N
		} else {
			intended = MOVE_S
		}
	}

	// If intended is a move, compute target from agent position and check age.
	if posv, ok := snapshot.(interface{ PositionValue() core.Position }); ok {
		if tgt, ok2 := computeTarget(posv.PositionValue(), intended); ok2 {
			if mt, found := o.memory.GetMemoryTile(tgt); found {
				age := tick - mt.LastSeen
				if age > CautionThreshold {
					// Prefer to OBSERVE to refresh belief instead of WAITing silently.
					return OBSERVE
				}
			}
		}
	}

	return intended
}

// Memory exposes the agent's memory for external inspection in tools/tests.
func (o *Oscillating) Memory() *Memory {
	return o.memory
}
