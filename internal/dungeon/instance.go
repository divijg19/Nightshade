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
}

func NewInstance(id string, anchor AnchorType) *Instance {
	return &Instance{
		ID:            id,
		Width:         7,
		Height:        7,
		Pressure:      0,
		MaxPressure:   20,
		ExitStability: 8,
		Entry:         core.Position{X: 3, Y: 6},
		Anchor:        core.Position{X: 3, Y: 3},
		Exit:          core.Position{X: 3, Y: 0},
		AnchorType:    anchor,
	}
}

func (d *Instance) Tick() {
	// v0.3.0 authoritative step: pressure is visible but has no gameplay effects yet.
	// Deterministic: increments by exactly +1 per tick while the agent is inside.
	d.Pressure++
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
