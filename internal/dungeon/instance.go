package dungeon

import (
	"fmt"
	"strings"

	"github.com/divijg19/Nightshade/internal/core"
)

type AnchorType string

const (
	AnchorMemoryVault  AnchorType = "MEMORY_VAULT"
	AnchorRecoveryNode AnchorType = "RECOVERY_NODE"
)

type Instance struct {
	ID            string
	Width         int
	Height        int
	Pressure      int
	MaxPressure   int
	ExitStability int

	Entry  core.Position
	Anchor core.Position
	Exit   core.Position

	AnchorType AnchorType
	AnchorUsed bool

	Done       bool
	DoneReason string

	// Decayed tracks tiles that have permanently decayed during the dungeon lifetime.
	Decayed map[core.Position]bool
	// Entities present in this dungeon instance (hostile, reactive)
	Entities []Entity

	// Objective system (v0.3.8)
	ObjectiveType      string        // STABILIZE | PURGE | RETRIEVE | HUNT
	ObjectiveProgress  int           // current progress toward objective
	ObjectiveTarget    int           // required progress to complete objective
	ObjectiveCompleted bool
	// PurgeNodes deterministically derived node positions for PURGE objectives
	PurgeNodes []core.Position
	// RetrieveHold is a simple counter for RETRIEVE hold ticks
	RetrieveHold int

	// Core integrity (0-100). When <=0 the dungeon collapses.
	CoreIntegrity int

	// Phase drives enrage behavior: NORMAL | ENRAGED
	Phase string

	// Exit channeling state (v0.3.9)
	ExitChanneling bool
	ExitChannelTick int

	// Deterministic risk nodes (v0.3.9)
	FragmentNode     core.Position
	CorruptionWell   core.Position
	OverchargeNode   core.Position
}

// EntityKind enumerates minimal enemy types.
type EntityKind string

const (
	EntityWraith  EntityKind = "WRAITH"
	EnemyHunter   EntityKind = "HUNTER"
	EnemySentinel EntityKind = "SENTINEL"
	EnemyWarden   EntityKind = "WARDEN"
	EnemyShade    EntityKind = "SHADE"
)

// Entity is a minimal, deterministic hostile presence living inside
// a dungeon instance.
type Entity struct {
	ID    string
	Pos   core.Position
	Kind  EntityKind
	Aggro int
	// TargetOverride is a temporary override: "anchor", "exit", "unknown"
	TargetOverride string
	// OverrideTicks remaining for the override
	OverrideTicks int
	// TargetLocked is a separate lock state (e.g., Sentinel locks).
	TargetLocked bool
	// LockTicks remaining for TargetLocked
	LockTicks int
	// LastMoveTick is the last global tick when this entity moved
	LastMoveTick int
	// LastKnown player position (when player was last visible to this entity)
	LastKnown    core.Position
	HasLastKnown bool
}

func NewInstance(id string, anchor AnchorType) *Instance {
	objectiveType := assignObjectiveType(id)
	purgeNodes := computePurgeNodes(id, 3)
	objectiveTarget := objectiveTargetForType(objectiveType, len(purgeNodes))
	inst := &Instance{
		ID:                 id,
		Width:              7,
		Height:             7,
		Pressure:           0,
		MaxPressure:        20,
		ExitStability:      8,
		Entry:              core.Position{X: 3, Y: 6},
		Anchor:             core.Position{X: 3, Y: 3},
		Exit:               core.Position{X: 3, Y: 0},
		AnchorType:         anchor,
		Decayed:            make(map[core.Position]bool),
		Entities:           []Entity{},
		ObjectiveType:      objectiveType,
		ObjectiveProgress:  0,
		ObjectiveTarget:    objectiveTarget,
		ObjectiveCompleted: false,
		PurgeNodes:         purgeNodes,
		RetrieveHold:       0,
		CoreIntegrity:      100,
		Phase:              "NORMAL",
		ExitChanneling:     false,
		ExitChannelTick:    0,
	}

	// compute deterministic risk nodes
	frag, well, over := computeRiskNodes(id)
	inst.FragmentNode = frag
	inst.CorruptionWell = well
	inst.OverchargeNode = over
	// If this instance is a HUNT objective, spawn a deterministic elite.
	if inst.ObjectiveType == "HUNT" {
		// Place elite near anchor deterministically.
		pos := inst.Anchor
		if pos.Y-1 >= 0 {
			pos = core.Position{X: pos.X, Y: pos.Y - 1}
		}
		e := Entity{ID: "elite-0", Pos: pos, Kind: EnemyHunter, Aggro: 0}
		inst.Entities = append(inst.Entities, e)
	}
	return inst
}

// assignObjectiveType deterministically maps an instance ID to one of four objectives.
func assignObjectiveType(id string) string {
	key := strings.TrimPrefix(id, "D-")
	sum := 0
	for i := 0; i < len(key); i++ {
		sum += int(key[i])
	}
	switch sum % 4 {
	case 0:
		return "STABILIZE"
	case 1:
		return "PURGE"
	case 2:
		return "RETRIEVE"
	default:
		return "HUNT"
	}
}

func objectiveTargetForType(objectiveType string, purgeNodeCount int) int {
	switch objectiveType {
	case "STABILIZE":
		return 5
	case "PURGE":
		if purgeNodeCount < 1 {
			return 1
		}
		return purgeNodeCount
	case "RETRIEVE":
		return 2
	case "HUNT":
		return 1
	default:
		return 1
	}
}

// computePurgeNodes deterministically produces `n` node positions inside the dungeon.
func computePurgeNodes(id string, n int) []core.Position {
	nodes := make([]core.Position, 0, n)
	seed := 0
	for i := 0; i < len(id); i++ {
		seed += int(id[i])
	}
	w := 7 - 2
	h := 7 - 2
	if w <= 0 {
		w = 1
	}
	if h <= 0 {
		h = 1
	}
	for i := 0; i < n; i++ {
		x := 1 + ((seed + i*3) % w)
		y := 1 + ((seed + i*7) % h)
		p := core.Position{X: x, Y: y}
		// avoid anchor/exit
		if p == (core.Position{X: 3, Y: 3}) || p == (core.Position{X: 3, Y: 0}) {
			p.X = (p.X % (7 - 2)) + 1
			p.Y = (p.Y % (7 - 2)) + 1
		}
		nodes = append(nodes, p)
	}
	return nodes
}

// AddDefaultEntities seeds the dungeon with the canonical single WRAITH.
// This deterministic placement puts the WRAITH at the anchor if free,
// otherwise adjacent above the anchor.
func (d *Instance) AddDefaultEntities() {
	// Seed canonical single Hunter for backward compatibility
	pos := d.Anchor
	if pos.Y-1 >= 0 {
		pos = core.Position{X: d.Anchor.X, Y: d.Anchor.Y - 1}
	}
	e := Entity{ID: "e-0", Pos: pos, Kind: EnemyHunter, Aggro: 0}
	d.Entities = append(d.Entities, e)
}

// SeedEntitiesForSignal deterministically composes entity list based on
// a simple hash of signalType and globalTick. It never uses RNG and
// produces positions within inner bounds avoiding anchor and exit.
func (d *Instance) SeedEntitiesForSignal(signalType string, globalTick int) {
	d.Entities = []Entity{}
	// simple seed: sum of bytes of signalType + globalTick
	seed := 0
	for i := 0; i < len(signalType); i++ {
		seed += int(signalType[i])
	}
	seed += globalTick

	// helper to produce a deterministic position
	makePos := func(offset int) core.Position {
		w := d.Width - 2
		h := d.Height - 2
		if w <= 0 {
			w = 1
		}
		if h <= 0 {
			h = 1
		}
		x := 1 + ((seed + offset*3) % w)
		y := 1 + ((seed + offset*7) % h)
		p := core.Position{X: x, Y: y}
		// avoid anchor/exit
		if p == d.Anchor || p == d.Exit {
			p.X = (p.X % (d.Width - 2)) + 1
			p.Y = (p.Y % (d.Height - 2)) + 1
		}
		return p
	}

	add := func(kind EntityKind, idOff int) {
		p := makePos(idOff)
		e := Entity{ID: fmt.Sprintf("%s-%d", string(kind), idOff), Pos: p, Kind: kind}
		d.Entities = append(d.Entities, e)
	}

	switch signalType {
	case "NULL":
		add(EnemyHunter, 0)
	case "FRACTURE":
		add(EnemyHunter, 0)
		add(EnemySentinel, 1)
	case "MEMORY_VAULT":
		add(EnemyWarden, 0)
		add(EnemySentinel, 1)
	case "RECOVERY_NODE":
		add(EnemyHunter, 0)
		add(EnemyShade, 1)
	default:
		// fallback: one hunter
		add(EnemyHunter, 0)
	}
}

func (d *Instance) Tick() {
	// v0.3.0 authoritative step: pressure is visible but has no gameplay effects yet.
	// Deterministic: increments by exactly +1 per tick while the agent is inside.
	d.Pressure++

	// v0.3.2: apply deterministic tile decay when pressure reaches unstable band (>=6).
	if d.Pressure >= 6 {
		for y := 1; y < d.Height-1; y++ {
			for x := 1; x < d.Width-1; x++ {
				p := core.Position{X: x, Y: y}
				if _, ok := d.Decayed[p]; ok {
					continue
				}
				if (x+y+d.Pressure)%4 == 0 {
					d.Decayed[p] = true
				}
			}
		}
	}
}

// TickWithAnchor advances the dungeon tick with awareness of whether the
// occupying agent is standing on the anchor tile. When `atAnchor` is true
// pressure increases only every 2 authoritative ticks (deterministic).
func (d *Instance) TickWithAnchor(atAnchor bool, globalTick int, cadence int) {
	if atAnchor {
		if cadence <= 0 {
			cadence = 2
		}
		if globalTick%cadence == 0 {
			d.Pressure++
			// apply decay same as normal Tick when pressure advances
			if d.Pressure >= 6 {
				for y := 1; y < d.Height-1; y++ {
					for x := 1; x < d.Width-1; x++ {
						p := core.Position{X: x, Y: y}
						if _, ok := d.Decayed[p]; ok {
							continue
						}
						if (x+y+d.Pressure)%4 == 0 {
							d.Decayed[p] = true
						}
					}
				}
			}
		}
		return
	}
	// default behavior
	d.Tick()
}

// InstabilityBand deterministically classifies the dungeon state from pressure.
// Bands:
// 0: 0-5 (Stable)
// 1: 6-10 (Unstable)
// 2: 11-15 (Dangerous)
// 3: 16+ (Critical)
func (d *Instance) InstabilityBand() int {
	p := d.Pressure
	switch {
	case p >= 16:
		return 3
	case p >= 11:
		return 2
	case p >= 6:
		return 1
	default:
		return 0
	}
}

func (d *Instance) InBounds(pos core.Position) bool {
	return pos.X >= 0 && pos.Y >= 0 && pos.X < d.Width && pos.Y < d.Height
}

func (d *Instance) GlyphAt(p core.Position) rune {
	if !d.InBounds(p) {
		return 0
	}

	// Template: solid walls at border, empty interior.
	if p.X == 0 || p.Y == 0 || p.X == d.Width-1 || p.Y == d.Height-1 {
		if p == d.Exit {
			return 'X'
		}
		if p == d.Entry {
			return 'E'
		}
		return '#'
	}

	if p == d.Anchor {
		return 'A'
	}
	// If tile decayed, show persistent '~' floor.
	if d.Decayed[p] {
		return '~'
	}
	return '.'
}

func (d *Instance) RenderASCII() string {
	var b strings.Builder
	for y := 0; y < d.Height; y++ {
		for x := 0; x < d.Width; x++ {
			b.WriteRune(d.GlyphAt(core.Position{X: x, Y: y}))
		}
		if y < d.Height-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func (d *Instance) String() string {
	return fmt.Sprintf("dungeon %s pressure=%d exit=%d", d.ID, d.Pressure, d.ExitStability)
}

// Enraged reports whether this instance is in enraged (critical) state.
func (d *Instance) Enraged() bool {
	return d.Pressure >= 16
}

// computeRiskNodes deterministically produces three distinct node positions
// for Fragment, Corruption Well, and Overcharge based on the instance id.
func computeRiskNodes(id string) (core.Position, core.Position, core.Position) {
	seed := 0
	for i := 0; i < len(id); i++ {
		seed += int(id[i])
	}
	w := 7 - 2
	h := 7 - 2
	if w <= 0 {
		w = 1
	}
	if h <= 0 {
		h = 1
	}
	a := core.Position{X: 1 + (seed%w), Y: 1 + ((seed*3)%h)}
	b := core.Position{X: 1 + ((seed+7)%w), Y: 1 + ((seed*5)%h)}
	c := core.Position{X: 1 + ((seed+13)%w), Y: 1 + ((seed*11)%h)}
	// avoid collisions with anchor/exit (3,3) and ensure distinctness
	safe := func(p core.Position) core.Position {
		if p == (core.Position{X: 3, Y: 3}) || p == (core.Position{X: 3, Y: 0}) {
			p.X = (p.X % (7-2)) + 1
			p.Y = (p.Y % (7-2)) + 1
		}
		return p
	}
	a = safe(a)
	b = safe(b)
	c = safe(c)
	// ensure uniqueness by nudging if equal
	if a == b {
		b.X = (b.X % (7-2)) + 1
	}
	if a == c || b == c {
		c.Y = (c.Y % (7-2)) + 1
	}
	return a, b, c
}
