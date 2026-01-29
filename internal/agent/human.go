package agent

import (
	"fmt"
	"os"
	"strings"

	"github.com/divijg19/Nightshade/internal/core"
)

// HumanInput is a package-level hook used to obtain a single line of
// input for the human agent. Tests may override this to provide
// deterministic, non-blocking input. By default it uses fmt.Scanln.
var HumanInput func() (string, error) = func() (string, error) {
	var s string
	_, err := fmt.Scanln(&s)
	return s, err
}

type Human struct {
	id     string
	memory *Memory
	energy int
	// replay buffer and state (human-only introspection replay)
	snaps        snapshotRing
	inReplay     bool
	replayCursor int // offset from newest (0=newest)
}

func NewHuman(id string) *Human {
	return &Human{id: id, memory: NewMemory(), energy: MaxEnergy}
}

func (h *Human) ID() string      { return h.id }
func (h *Human) Memory() *Memory { return h.memory }
func (h *Human) Energy() int     { return h.energy }

// visibility radius must match the runtime default (kept as literal to
// avoid touching runtime package). This mirrors runtime.defaultVisibilityRadius.
const humanVisibilityRadius = 2

func keyToAction(key string) Action {
	if key == "" {
		return WAIT
	}
	r := []rune(key)[0]
	switch r {
	case 'w':
		return MOVE_N
	case 's':
		return MOVE_S
	case 'a':
		return MOVE_W
	case 'd':
		return MOVE_E
	case 'e':
		return OBSERVE
	case '.':
		return WAIT
	case 'q':
		return WAIT
	default:
		return WAIT
	}
}

func (h *Human) Decide(snapshot Snapshot) Action {
	// 1. Update memory only from what is currently visible and capture previous MemoryTile map.
	var prev map[core.Position]MemoryTile
	if h.memory != nil {
		prev = h.memory.UpdateFromVisible(snapshot)
	}

	// 2. Emit belief signal derived from memory for contagion.
	pos := core.Position{}
	if p, ok := snapshot.(interface{ PositionValue() core.Position }); ok {
		pos = p.PositionValue()
	}
	beliefs := []Belief{}
	for _, mt := range h.memory.All() {
		age := 0
		if t, ok := snapshot.(interface{ TickValue() int }); ok {
			age = t.TickValue() - mt.LastSeen
		}
		beliefs = append(beliefs, Belief{Tile: mt.Tile, Age: age, ScarLevel: mt.ScarLevel})
	}
	tick := 0
	if t, ok := snapshot.(interface{ TickValue() int }); ok {
		tick = t.TickValue()
	}
	emitBeliefSignal(h.id, tick, pos, beliefs)

	// 3. Apply contagion
	_ = applyBeliefContagion(h.id, pos, tick, h.memory, h.energy)

	// 4. Detect & apply conflicts
	detectAndApplyConflicts(h.memory, prev, tick)

	// 5. Build Observation
	effectiveParanoia := ParanoiaThreshold
	effectiveCaution := CautionThreshold
	if h.energy < LowEnergyThreshold {
		effectiveParanoia = ParanoiaThreshold - 2
		effectiveCaution = CautionThreshold - 1
	}
	obs := buildObservation(h.memory, snapshot, prev, h.energy, effectiveParanoia)

	// 6. Render Observation to terminal (viewport centered on agent)
	visMap := map[core.Position]rune{}
	for _, v := range obs.Visible {
		visMap[v.Position] = v.Glyph
	}
	center := core.Position{X: 0, Y: 0}
	if p, ok := snapshot.(interface{ PositionValue() core.Position }); ok {
		center = p.PositionValue()
	}
	r := humanVisibilityRadius
	for dy := -r; dy <= r; dy++ {
		line := ""
		for dx := -r; dx <= r; dx++ {
			pos := core.Position{X: center.X + dx, Y: center.Y + dy}
			if dx == 0 && dy == 0 {
				line += "@"
				continue
			}
			if g, ok := visMap[pos]; ok {
				if g == 0 {
					line += "."
				} else {
					line += string(g)
				}
			} else {
				line += "?"
			}
		}
		fmt.Println(line)
	}

	// 6. Render cognitive HUD and perceptual narration (use perception.Describe)
	sumScars := 0
	for _, mt := range h.memory.All() {
		sumScars += mt.ScarLevel
	}
	st := ReadOnlyAgentState{
		Energy:            h.energy,
		EffectiveParanoia: effectiveParanoia,
		EffectiveCaution:  effectiveCaution,
		SumScars:          sumScars,
		BeliefCount:       h.memory.Count(),
		Position:          center,
		Tick:              obs.Tick,
	}
	// Describe returns short deterministic sentences; print them instead of raw tiles
	for _, line := range Describe(obs, st) {
		fmt.Println(line)
	}
	// Also print minimal HUD (numbers preserved as before)
	fmt.Printf("Energy: %d/%d\n", h.energy, MaxEnergy)
	fmt.Printf("Paranoia: %d\n", effectiveParanoia)
	fmt.Printf("Scars: %d\n", sumScars)
	fmt.Printf("Beliefs: %d\n", h.memory.Count())

	// 8. Read human input (support INTROSPECT 'i' as a read-only, non-advancing affordance)
	input := ""
	for {
		if HumanInput != nil {
			in, err := HumanInput()
			if err == nil {
				input = strings.TrimSpace(in)
			}
		}
		if input == "q" {
			os.Exit(0)
		}
		// INTROSPECT summary (read-only)
		if input == "i" {
			// INTROSPECT: render introspection report (read-only) and loop to read input again.
			// Do not mutate memory, energy, or advance time here.
			rpt := Introspect(*h.memory, obs.Tick)
			fmt.Println("You pause and examine your thoughts.")
			fmt.Printf("\nBeliefs held: %d\n", rpt.TotalBeliefs)
			fmt.Printf("Certain: %d\n", rpt.Certain)
			fmt.Printf("Recent: %d\n", rpt.Recent)
			fmt.Printf("Fading: %d\n", rpt.Fading)
			fmt.Printf("Doubtful: %d\n", rpt.Doubtful)
			if rpt.HasScars {
				fmt.Println("\nSome memories feel unreliable.")
			} else {
				fmt.Println("\nYour thoughts feel settled.")
			}
			// After rendering introspection, continue loop to read next input.
			input = ""
			continue
		}
		// Replay navigation: '[' back, ']' forward. Replay is human-only and read-only.
		if input == "[" || input == "]" {
			// enter replay mode if there are snapshots
			if h.snaps.len() == 0 {
				input = ""
				continue
			}
			h.inReplay = true
			if input == "[" {
				// move cursor backward (older)
				if h.replayCursor+1 < h.snaps.len() {
					h.replayCursor++
				}
			} else {
				// ']' move forward (towards newest)
				if h.replayCursor > 0 {
					h.replayCursor--
				}
			}
			// display snapshot at cursor
			if snap, ok := h.snaps.getFromNewest(h.replayCursor); ok {
				fmt.Printf("You revisit an earlier state of mind. (Tick %d)\n", snap.Tick)
				fmt.Printf("\nBeliefs held: %d\n", snap.Report.TotalBeliefs)
				fmt.Printf("Certain: %d\n", snap.Report.Certain)
				fmt.Printf("Recent: %d\n", snap.Report.Recent)
				fmt.Printf("Fading: %d\n", snap.Report.Fading)
				fmt.Printf("Doubtful: %d\n", snap.Report.Doubtful)
				if snap.Report.HasScars {
					fmt.Println("\nSome memories feel unreliable.")
				} else {
					fmt.Println("\nYour thoughts feel settled.")
				}
			}
			// continue reading input while in replay
			input = ""
			continue
		}
		// any other key (movement/action) exits replay mode if active and proceeds
		if h.inReplay {
			// exit replay: reset cursor to newest
			h.inReplay = false
			h.replayCursor = 0
		}
		break
	}

	// 9. Translate to intended Action
	intended := keyToAction(input)

	// 10. Apply caution override
	if intended == MOVE_N || intended == MOVE_S || intended == MOVE_E || intended == MOVE_W {
		if posv, ok := snapshot.(interface{ PositionValue() core.Position }); ok {
			if tgt, ok2 := computeTarget(posv.PositionValue(), intended); ok2 {
				if mt, found := h.memory.GetMemoryTile(tgt); found {
					age := obs.Tick - mt.LastSeen
					if age > effectiveCaution {
						intended = OBSERVE
					}
				}
			}
		}
	}

	// 11. Critical energy collapse
	final := intended
	if h.energy < CriticalEnergyThreshold {
		final = WAIT
	}

	// 12. Apply energy effects
	switch final {
	case MOVE_N, MOVE_S, MOVE_E, MOVE_W:
		h.energy -= MoveEnergyCost
	case OBSERVE:
		h.energy -= ObserveEnergyCost
	case WAIT:
		h.energy += WaitEnergyRestore
	}
	if h.energy > MaxEnergy {
		h.energy = MaxEnergy
	}
	if h.energy < MinEnergy {
		h.energy = MinEnergy
	}

	// 13. OBSERVE healing
	if final == OBSERVE && h.memory != nil {
		for pos, mt := range h.memory.tiles {
			if mt.ScarLevel > 0 {
				mt.ScarLevel -= 1
				if mt.ScarLevel < 0 {
					mt.ScarLevel = 0
				}
				h.memory.tiles[pos] = mt
			}
		}
	}

	// Snapshot capture: append introspection snapshot after a real action resolves.
	// Do not capture snapshots while user is in replay mode (replay is read-only).
	if !h.inReplay {
		snap := IntrospectionSnapshot{Tick: obs.Tick, Report: Introspect(*h.memory, obs.Tick)}
		h.snaps.append(snap)
	}

	return final
}
