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

// buildObservation constructs an Observation from the given memory and
// runtime snapshot. It injects hallucinated tiles from memory when a
// MemoryTile's Age > ParanoiaThreshold. Hallucinated tiles come only from
// memory and are not used to update memory.
// buildObservation constructs an Observation from the given memory and
// runtime snapshot. It injects hallucinated tiles from memory when a
// MemoryTile's Age > effectiveParanoia. If prevLastSeen is provided it
// is used to preserve hallucination state across an UpdateFromVisible
// (useful when energy is critical and OBSERVE should not clear hallucinations).
func buildObservation(mem *Memory, snapshot interface{}, prevLastSeen map[core.Position]int, energy int, effectiveParanoia int) Observation {
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
	visMap := make(map[core.Position]struct{})
	for _, vv := range vis {
		visMap[vv.Position] = struct{}{}
	}
	if mem != nil {
		for _, mt := range mem.All() {
			age := tick - mt.LastSeen
			known = append(known, Belief{Tile: mt.Tile, Age: age})

			// Determine whether to inject a hallucination. Normally we
			// inject when current age > effectiveParanoia. However, when
			// energy is critical, OBSERVE may have just updated LastSeen;
			// use prevLastSeen to decide if the belief was hallucinated
			// before the update and should remain injected.
			inject := false
			if age > effectiveParanoia {
				inject = true
			} else if energy < CriticalEnergyThreshold {
				if p, ok := prevLastSeen[mt.Tile.Position]; ok && p >= 0 {
					if (tick - p) > effectiveParanoia {
						inject = true
					}
				}
			}

			if inject {
				if _, ok := visMap[mt.Tile.Position]; !ok {
					vis = append(vis, mt.Tile)
					visMap[mt.Tile.Position] = struct{}{}
				}
			}
		}
	}

	return Observation{Visible: vis, Known: known, Tick: tick}
}

type Scripted struct {
	id     string
	memory *Memory
	energy int
}

func NewScripted(id string) *Scripted {
	return &Scripted{id: id,
		memory: NewMemory(),
		energy: MaxEnergy,
	}
}

func (s *Scripted) ID() string {
	return s.id
}

func (s *Scripted) Decide(snapshot Snapshot) Action {
	// Update memory only from what is currently visible and capture previous LastSeen map.
	var prev map[core.Position]int
	if s.memory != nil {
		prev = s.memory.UpdateFromVisible(snapshot)
	}

	// Compute effective thresholds based on energy
	effectiveParanoia := ParanoiaThreshold
	effectiveCaution := CautionThreshold
	if s.energy < LowEnergyThreshold {
		effectiveParanoia = ParanoiaThreshold - 2
		effectiveCaution = CautionThreshold - 1
	}

	// Build agent-side Observation using helper (includes hallucinations).
	obs := buildObservation(s.memory, snapshot, prev, s.energy, effectiveParanoia)

	// Decision flow: compute intended action (existing behavior), then
	// potentially override with OBSERVE if target belief is stale.
	intended := MOVE_E

	// If intended is a move, compute target from agent's current position and
	// mark that we should OBSERVE instead of moving if the target belief is stale.
	if posv, ok := snapshot.(interface{ PositionValue() core.Position }); ok {
		if tgt, ok2 := computeTarget(posv.PositionValue(), intended); ok2 {
			if mt, found := s.memory.GetMemoryTile(tgt); found {
				age := obs.Tick - mt.LastSeen
				if age > effectiveCaution {
					// Prefer to OBSERVE (refresh visible belief) instead of a blind WAIT.
					intended = OBSERVE
				}
			}
		}
	}

	// Critical energy collapse: force WAIT
	final := intended
	if s.energy < CriticalEnergyThreshold {
		final = WAIT
	}

	// Apply energy effects after final action selection
	switch final {
	case MOVE_N, MOVE_S, MOVE_E, MOVE_W:
		s.energy -= MoveEnergyCost
	case OBSERVE:
		s.energy -= ObserveEnergyCost
	case WAIT:
		s.energy += WaitEnergyRestore
	}
	if s.energy > MaxEnergy {
		s.energy = MaxEnergy
	}
	if s.energy < MinEnergy {
		s.energy = MinEnergy
	}

	_ = obs
	return final
}

// Memory exposes the agent's memory for external inspection in tools/tests.
func (s *Scripted) Memory() *Memory {
	return s.memory
}

// Energy returns the current energy level for debug/inspection.
func (s *Scripted) Energy() int { return s.energy }

// Oscillating moves north on even ticks and south on odd ticks.
type Oscillating struct {
	id     string
	memory *Memory
	energy int
}

func NewOscillating(id string) *Oscillating {
	return &Oscillating{id: id,
		memory: NewMemory(),
		energy: MaxEnergy,
	}
}

func (o *Oscillating) ID() string {
	return o.id
}

func (o *Oscillating) Decide(snapshot Snapshot) Action {
	// Update memory from visible tiles only and capture previous LastSeen
	var prev map[core.Position]int
	if o.memory != nil {
		prev = o.memory.UpdateFromVisible(snapshot)
	}

	effectiveParanoia := ParanoiaThreshold
	effectiveCaution := CautionThreshold
	if o.energy < LowEnergyThreshold {
		effectiveParanoia = ParanoiaThreshold - 2
		effectiveCaution = CautionThreshold - 1
	}

	// Build agent-side Observation using helper (includes hallucinations).
	obs := buildObservation(o.memory, snapshot, prev, o.energy, effectiveParanoia)
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
				age := obs.Tick - mt.LastSeen
				if age > effectiveCaution {
					// Prefer to OBSERVE to refresh belief instead of WAITing silently.
					intended = OBSERVE
				}
			}
		}
	}

	final := intended
	if o.energy < CriticalEnergyThreshold {
		final = WAIT
	}

	switch final {
	case MOVE_N, MOVE_S, MOVE_E, MOVE_W:
		o.energy -= MoveEnergyCost
	case OBSERVE:
		o.energy -= ObserveEnergyCost
	case WAIT:
		o.energy += WaitEnergyRestore
	}
	if o.energy > MaxEnergy {
		o.energy = MaxEnergy
	}
	if o.energy < MinEnergy {
		o.energy = MinEnergy
	}

	return final
}

// Memory exposes the agent's memory for external inspection in tools/tests.
func (o *Oscillating) Memory() *Memory {
	return o.memory
}

// Energy returns the current energy level for debug/inspection.
func (o *Oscillating) Energy() int { return o.energy }
