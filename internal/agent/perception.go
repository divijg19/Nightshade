package agent

import "github.com/divijg19/Nightshade/internal/core"

// Perceptual Narration Layer
// Rule table (explicit, ordered):
// 1. Newly visible tiles: any Known belief with Age==0 -> "You notice something nearby."
// 2. Hallucinated tile: Belief present in Observation.Visible but Age > effectiveParanoia -> "Something feels wrong here."
// 3. Belief only: any Known belief with Age > effectiveParanoia -> "You think something might be there."
// 4. Scar present: any Known.ScarLevel >= 1 -> "A familiar unease tightens."
// 5. Low energy: Energy < LowEnergyThreshold -> "Your thoughts feel sluggish."
// 6. Critical energy: Energy < CriticalEnergyThreshold -> "You can't trust your instincts right now."
// 7. OBSERVE cue: if any movement target is stale (age > effectiveCaution) -> "You steady your breathing and focus."
// Notes:
// - These mappings are deterministic and purely translational.
// - No memory mutations or world changes are performed here.
// - The function is pure and returns 1..7 short lines (max 7).

// ReadOnlyAgentState is a minimal read-only view passed to Describe.
type ReadOnlyAgentState struct {
	Energy           int
	EffectiveParanoia int
	EffectiveCaution  int
	SumScars         int
	BeliefCount      int
	Position         core.Position
	Tick             int
}

// Describe translates an Observation and agent read-only state into
// a short deterministic list of perceptual sentences. Pure function.
func Describe(ob Observation, st ReadOnlyAgentState) []string {
	lines := []string{}
	// helpers
	visMap := map[core.Position]struct{}{}
	for _, v := range ob.Visible { visMap[v.Position] = struct{}{} }

	// 1. Newly visible tiles: Known Age == 0
	for _, k := range ob.Known {
		if k.Age == 0 {
			lines = append(lines, "You notice something nearby.")
			break
		}
	}

	// 2. Hallucinated tiles: visible but their known Age > effectiveParanoia
	for _, k := range ob.Known {
		if k.Age > st.EffectiveParanoia {
			if _, ok := visMap[k.Tile.Position]; ok {
				lines = append(lines, "Something feels wrong here.")
				break
			}
		}
	}

	// 3. Belief-only (non-visible) strong but below hallucination: Age > effectiveParanoia
	for _, k := range ob.Known {
		if k.Age > st.EffectiveParanoia {
			lines = append(lines, "You think something might be there.")
			break
		}
	}

	// 4. Scar present
	for _, k := range ob.Known {
		if k.ScarLevel >= 1 {
			lines = append(lines, "A familiar unease tightens.")
			break
		}
	}

	// 5/6. Energy cues
	if st.Energy < CriticalEnergyThreshold {
		lines = append(lines, "You can't trust your instincts right now.")
	} else if st.Energy < LowEnergyThreshold {
		lines = append(lines, "Your thoughts feel sluggish.")
	}

	// 7. OBSERVE cue: if there exists a remembered movement target whose age > effectiveCaution
	// We determine this by scanning Known beliefs for any position that would be a movement target
	// relative to the agent position. This is an approximation used for deterministic narration
	// without changing cognition. It does not alter agent decisions.
	foundStaleTarget := false
	for _, k := range ob.Known {
		// consider neighbor positions only
		dx := k.Tile.Position.X - st.Position.X
		dy := k.Tile.Position.Y - st.Position.Y
		manh := dx
		if manh < 0 { manh = -manh }
		ady := dy
		if ady < 0 { ady = -ady }
		if manh+ady == 1 {
			if k.Age > st.EffectiveCaution {
				foundStaleTarget = true
				break
			}
		}
	}
	if foundStaleTarget {
		lines = append(lines, "You steady your breathing and focus.")
	}

	// Truncate to max 7 lines and return
	if len(lines) > 7 { lines = lines[:7] }
	if len(lines) == 0 { lines = append(lines, "You sense nothing unusual.") }
	return lines
}
