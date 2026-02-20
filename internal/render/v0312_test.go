package render

import (
	"strings"
	"testing"

	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/core"
)

func testGrid12() [][]rune {
	return [][]rune{
		[]rune("#######"),
		[]rune("#.....#"),
		[]rune("#..A..#"),
		[]rune("#.....#"),
		[]rune("###X###"),
	}
}

func TestV0312_DungeonHierarchyOrder(t *testing.T) {
	obs := agent.Observation{
		Tick:     10,
		Mode:     "dungeon",
		Position: core.Position{X: 1, Y: 1},
		Event:    "Event line",
		Dungeon: &agent.DungeonView{
			Grid:            testGrid12(),
			Pressure:        6,
			MaxPressure:     20,
			CoreIntegrity:   90,
			Threat:          "MEDIUM",
			InstabilityBand: 1,
			PathType:        "STABILIZER",
			Phase:           "II",
			AbilityName:     "SUPPRESS",
			AbilityCooldown: 3,
			ExitChanneling:  true,
			ExitChannelTick: 9,
		},
	}
	plain := stripANSI11(RenderForTest(obs))
	ixPath := strings.Index(plain, "PATH:")
	ixPhase := strings.Index(plain, "PHASE:")
	ixPressure := strings.Index(plain, "PRESSURE:")
	ixAbility := strings.Index(plain, "ABILITY:")
	ixEvent := strings.Index(plain, "Event line")
	ixGrid := strings.Index(plain, "◉")
	ixChannel := strings.Index(plain, "EXIT CHANNEL:")
	ixHUD := strings.Index(plain, "Energy 100/100")
	if !(ixPath < ixPhase && ixPhase < ixPressure && ixPressure < ixAbility && ixAbility < ixEvent && ixEvent < ixGrid && ixGrid < ixChannel && ixChannel < ixHUD) {
		t.Fatalf("unexpected dungeon hierarchy order")
	}
	if strings.Contains(plain, "\r\n\r\n") {
		t.Fatalf("expected no extra blank lines in dungeon frame")
	}
}

func TestV0312_PhaseVisualEvolutionDeterministic(t *testing.T) {
	base := agent.Observation{
		Tick:     4,
		Mode:     "dungeon",
		Position: core.Position{X: 1, Y: 1},
		Dungeon: &agent.DungeonView{
			Grid:            testGrid12(),
			Pressure:        8,
			MaxPressure:     20,
			CoreIntegrity:   85,
			Threat:          "HIGH",
			InstabilityBand: 2,
			PathType:        "AGGRESSOR",
			AbilityName:     "OVERDRIVE",
		},
	}
	p1 := base
	p1.Dungeon.Phase = "I"
	o1 := RenderForTest(p1)
	p2 := base
	p2.Dungeon.Phase = "II"
	o2 := RenderForTest(p2)
	if strings.Contains(o1, ANSIYellowBright+"◉"+ANSIReset) {
		t.Fatalf("phase I should not use phase II core highlight")
	}
	if !strings.Contains(o2, ANSIYellowBright+"◉"+ANSIReset) {
		t.Fatalf("phase II should brighten core glyph")
	}
	p3 := base
	p3.Dungeon.Phase = "III"
	o3 := RenderForTest(p3)
	if !strings.Contains(o3, ANSIRedBright+"PRESSURE:") {
		t.Fatalf("phase III should tint pressure red")
	}
	p3b := p3
	p3b.Tick = p3.Tick + 1
	o3b := RenderForTest(p3b)
	if o3 == o3b {
		t.Fatalf("phase III alternation should deterministically vary with tick")
	}
}

func TestV0312_BoardBiasRendering(t *testing.T) {
	obs := agent.Observation{
		Tick: 220,
		Mode: "board",
		Board: &agent.BoardView{Signals: []agent.SignalView{
			{ID: "S000", Type: "FRACTURE", Decay: 6, Corruption: 3, SignalBias: "AGGRESSOR"},
		}},
	}
	plain := stripANSI11(RenderForTest(obs))
	if !strings.Contains(plain, "(Aggressor Bias)") {
		t.Fatalf("expected signal bias telegraph in board line")
	}
}
