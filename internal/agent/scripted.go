package agent

import "github.com/divijg19/Nightshade/internal/core"

// BeliefSignal is an agent-local emission used for contagion among agents
// within a tick. It is stored in a package-level registry keyed by agent
// ID and is replaced each tick.
type BeliefSignal struct {
	Position core.Position
	Beliefs  []Belief
}

var beliefSignals = map[string]BeliefSignal{}
var beliefSignalsTick = -1

func manhattan(a, b core.Position) int {
	dx := a.X - b.X
	if dx < 0 {
		dx = -dx
	}
	dy := a.Y - b.Y
	if dy < 0 {
		dy = -dy
	}
	return dx + dy
}

// emitBeliefSignal stores this agent's belief signal for the current tick.
func emitBeliefSignal(id string, tick int, pos core.Position, beliefs []Belief) {
	if beliefSignalsTick != tick {
		beliefSignals = map[string]BeliefSignal{}
		beliefSignalsTick = tick
	}
	beliefSignals[id] = BeliefSignal{Position: pos, Beliefs: beliefs}
}

// applyBeliefContagion applies signals emitted by other agents to the
// receiver's memory according to the contagion rules. Returns a list of
// positions that were transferred (for debug/tests).
func applyBeliefContagion(receiverID string, receiverPos core.Position, tick int, receiverMem *Memory, receiverEnergy int) []core.Position {
	applied := []core.Position{}
	for senderID, sig := range beliefSignals {
		if senderID == receiverID {
			continue
		}
		if manhattan(sig.Position, receiverPos) > BeliefRadius {
			continue
		}
		for _, b := range sig.Beliefs {
			pos := b.Tile.Position
			// Determine sender's LastSeen from its belief Age: senderLastSeen = tick - b.Age
			senderLastSeen := tick - b.Age
			// Receiver's current belief
			if receiverMem == nil {
				continue
			}
			if cur, ok := receiverMem.GetMemoryTile(pos); ok {
				receiverLastSeen := cur.LastSeen
				// Eligibility: receiver does not have a newer belief OR low energy
				if !(receiverLastSeen < senderLastSeen || receiverEnergy < LowEnergyThreshold) {
					continue
				}
				// Asymmetric dominance: compare strengths using ScarLevel
				ageA := tick - senderLastSeen
				ageB := tick - receiverLastSeen
				strengthA := ParanoiaThreshold - ageA
				strengthB := ParanoiaThreshold - ageB
				if strengthA < 0 {
					strengthA = 0
				}
				if strengthB < 0 {
					strengthB = 0
				}
				// include scar levels
				strengthA += b.ScarLevel
				strengthB += cur.ScarLevel
				if strengthA <= strengthB {
					continue
				}
			}
			// Apply transfer: insert into receiver memory with weakened LastSeen
			// Preserve existing ScarLevel if present
			scar := 0
			if cur, ok := receiverMem.GetMemoryTile(pos); ok {
				scar = cur.ScarLevel
			}
			receiverMem.tiles[pos] = MemoryTile{Tile: b.Tile, LastSeen: tick - TransferPenalty, ScarLevel: scar}
			applied = append(applied, pos)
		}
	}
	return applied
}

// detectAndApplyConflicts examines memory changes (prev map returned by
// UpdateFromVisible) and current memory to find conflicting beliefs and
// applies scars deterministically when conflict strength thresholds are met.
func detectAndApplyConflicts(mem *Memory, prev map[core.Position]MemoryTile, tick int) {
	if mem == nil {
		return
	}
	for pos, newMt := range mem.tiles {
		if oldMt, ok := prev[pos]; ok {
			if oldMt.Tile.Glyph != newMt.Tile.Glyph {
				// compute ages
				ageOld := tick - oldMt.LastSeen
				ageNew := tick - newMt.LastSeen
				strOld := ParanoiaThreshold - ageOld
				strNew := ParanoiaThreshold - ageNew
				if strOld < 0 {
					strOld = 0
				}
				if strNew < 0 {
					strNew = 0
				}
				strOld += oldMt.ScarLevel
				strNew += newMt.ScarLevel
				if strOld >= ConflictThreshold && strNew >= ConflictThreshold {
					// Apply scar to the current memory entry
					nm := mem.tiles[pos]
					nm.ScarLevel += 1
					if nm.LastSeen < tick-ScarPenalty {
						nm.LastSeen = tick - ScarPenalty
					}
					mem.tiles[pos] = nm
				}
			}
		}
	}
}

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
func buildObservation(mem *Memory, snapshot interface{}, prevLastSeen map[core.Position]MemoryTile, energy int, effectiveParanoia int) Observation {
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
			known = append(known, Belief{Tile: mt.Tile, Age: age, ScarLevel: mt.ScarLevel})

			// Determine per-tile effective paranoia considering scars and energy.
			// Use the caller-provided `effectiveParanoia` as the base threshold
			// (it already accounts for agent energy), then adjust by scar level.
			perTileParanoia := effectiveParanoia - mt.ScarLevel
			if energy < LowEnergyThreshold {
				perTileParanoia -= 2
			}

			inject := false
			if age > perTileParanoia {
				inject = true
			} else if energy < CriticalEnergyThreshold {
				if pmt, ok := prevLastSeen[mt.Tile.Position]; ok {
					// prevLastSeen stores previous MemoryTile; use its LastSeen
					// to determine whether hallucination should be preserved
					// under critical energy.
					if (tick - pmt.LastSeen) > perTileParanoia {
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
	// Update memory only from what is currently visible and capture previous
	// MemoryTile map (contains previous LastSeen and Tile when present).
	var prev map[core.Position]MemoryTile
	if s.memory != nil {
		prev = s.memory.UpdateFromVisible(snapshot)
	}

	// Emit belief signal derived from memory (known beliefs) for contagion.
	pos := core.Position{}
	if p, ok := snapshot.(interface{ PositionValue() core.Position }); ok {
		pos = p.PositionValue()
	}
	beliefs := []Belief{}
	for _, mt := range s.memory.All() {
		age := 0
		if t, ok := snapshot.(interface{ TickValue() int }); ok {
			age = t.TickValue() - mt.LastSeen
		}
		beliefs = append(beliefs, Belief{Tile: mt.Tile, Age: age})
	}
	// Use snapshot tick for registry
	tick := 0
	if t, ok := snapshot.(interface{ TickValue() int }); ok {
		tick = t.TickValue()
	}
	emitBeliefSignal(s.id, tick, pos, beliefs)

	// Apply contagion from earlier emitters in this tick (asymmetric)
	_ = applyBeliefContagion(s.id, pos, tick, s.memory, s.energy)

	// After contagion, detect conflicts and apply scars deterministically.
	detectAndApplyConflicts(s.memory, prev, tick)

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

	// OBSERVE healing: reduce ScarLevel by 1 for scarred memories when agent
	// successfully performs OBSERVE (partial healing per tick).
	if final == OBSERVE && s.memory != nil {
		for pos, mt := range s.memory.tiles {
			if mt.ScarLevel > 0 {
				mt.ScarLevel -= 1
				if mt.ScarLevel < 0 {
					mt.ScarLevel = 0
				}
				s.memory.tiles[pos] = mt
			}
		}
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
	// Update memory from visible tiles only and capture previous MemoryTile
	// map (contains previous LastSeen and Tile when present).
	var prev map[core.Position]MemoryTile
	if o.memory != nil {
		prev = o.memory.UpdateFromVisible(snapshot)
	}

	// Emit belief signal for contagion
	opos := core.Position{}
	if p, ok := snapshot.(interface{ PositionValue() core.Position }); ok {
		opos = p.PositionValue()
	}
	obeliefs := []Belief{}
	for _, mt := range o.memory.All() {
		age := 0
		if t, ok := snapshot.(interface{ TickValue() int }); ok {
			age = t.TickValue() - mt.LastSeen
		}
		obeliefs = append(obeliefs, Belief{Tile: mt.Tile, Age: age})
	}
	tick := 0
	if t, ok := snapshot.(interface{ TickValue() int }); ok {
		tick = t.TickValue()
	}
	emitBeliefSignal(o.id, tick, opos, obeliefs)
	_ = applyBeliefContagion(o.id, opos, tick, o.memory, o.energy)

	// After contagion, detect conflicts and apply scars deterministically.
	detectAndApplyConflicts(o.memory, prev, tick)

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
