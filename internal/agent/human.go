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
}

func NewHuman(id string) *Human {
	return &Human{id: id, memory: NewMemory(), energy: MaxEnergy}
}

func (h *Human) ID() string { return h.id }
func (h *Human) Memory() *Memory { return h.memory }
func (h *Human) Energy() int { return h.energy }

// visibility radius must match the runtime default (kept as literal to
// avoid touching runtime package). This mirrors runtime.defaultVisibilityRadius.
const humanVisibilityRadius = 2

func keyToAction(key string) Action {
	if key == "" { return WAIT }
	r := []rune(key)[0]
	switch r {
	case 'w': return MOVE_N
	case 's': return MOVE_S
	case 'a': return MOVE_W
	case 'd': return MOVE_E
	case 'e': return OBSERVE
	case '.': return WAIT
	case 'q': return WAIT
	default: return WAIT
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
	if t, ok := snapshot.(interface{ TickValue() int }); ok { tick = t.TickValue() }
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
	for _, v := range obs.Visible { visMap[v.Position] = v.Glyph }
	center := core.Position{X:0, Y:0}
	if p, ok := snapshot.(interface{ PositionValue() core.Position }); ok { center = p.PositionValue() }
	r := humanVisibilityRadius
	for dy := -r; dy <= r; dy++ {
		line := ""
		for dx := -r; dx <= r; dx++ {
			pos := core.Position{X: center.X + dx, Y: center.Y + dy}
			if dx == 0 && dy == 0 { line += "@"; continue }
			if g, ok := visMap[pos]; ok {
				if g == 0 { line += "." } else { line += string(g) }
			} else { line += "?" }
		}
		fmt.Println(line)
	}

	// 7. Render cognitive HUD
	sumScars := 0
	for _, mt := range h.memory.All() { sumScars += mt.ScarLevel }
	fmt.Printf("Energy: %d/%d\n", h.energy, MaxEnergy)
	fmt.Printf("Paranoia: %d\n", effectiveParanoia)
	fmt.Printf("Scars: %d\n", sumScars)
	fmt.Printf("Beliefs: %d\n", h.memory.Count())

	// 8. Read human input
	input := ""
	if HumanInput != nil {
		in, err := HumanInput()
		if err == nil { input = strings.TrimSpace(in) }
	}

	// 9. Translate to intended Action
	if input == "q" { os.Exit(0) }
	intended := keyToAction(input)

	// 10. Apply caution override
	if intended == MOVE_N || intended == MOVE_S || intended == MOVE_E || intended == MOVE_W {
		if posv, ok := snapshot.(interface{ PositionValue() core.Position }); ok {
			if tgt, ok2 := computeTarget(posv.PositionValue(), intended); ok2 {
				if mt, found := h.memory.GetMemoryTile(tgt); found {
					age := obs.Tick - mt.LastSeen
					if age > effectiveCaution { intended = OBSERVE }
				}
			}
		}
	}

	// 11. Critical energy collapse
	final := intended
	if h.energy < CriticalEnergyThreshold { final = WAIT }

	// 12. Apply energy effects
	switch final {
	case MOVE_N, MOVE_S, MOVE_E, MOVE_W: h.energy -= MoveEnergyCost
	case OBSERVE: h.energy -= ObserveEnergyCost
	case WAIT: h.energy += WaitEnergyRestore
	}
	if h.energy > MaxEnergy { h.energy = MaxEnergy }
	if h.energy < MinEnergy { h.energy = MinEnergy }

	// 13. OBSERVE healing
	if final == OBSERVE && h.memory != nil {
		for pos, mt := range h.memory.tiles {
			if mt.ScarLevel > 0 {
				mt.ScarLevel -= 1
				if mt.ScarLevel < 0 { mt.ScarLevel = 0 }
				h.memory.tiles[pos] = mt
			}
		}
	}

	return final
}
