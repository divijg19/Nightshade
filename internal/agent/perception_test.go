package agent

import (
	"strings"
	"testing"

	"github.com/divijg19/Nightshade/internal/core"
)

func TestDescribeLineCountWithinBounds(t *testing.T) {
	obs := Observation{Visible: []core.TileView{}, Known: []Belief{}, Tick: 1}
	st := ReadOnlyAgentState{Energy: MaxEnergy, EffectiveParanoia: ParanoiaThreshold, EffectiveCaution: CautionThreshold, SumScars: 0, BeliefCount: 0, Position: core.Position{X:0,Y:0}, Tick: 1}
	lines := Describe(obs, st)
	if len(lines) < 1 || len(lines) > 7 {
		t.Fatalf("Describe returned %d lines; expected 1..7", len(lines))
	}
}

func TestDescribeHallucinationAndBelief(t *testing.T) {
	// setup a known belief with old age, considered hallucinated
	k := Belief{Tile: core.TileView{Position: core.Position{X:1,Y:0}}, Age: ParanoiaThreshold + 2, ScarLevel: 0}
	obs := Observation{Visible: []core.TileView{{Position: core.Position{X:1,Y:0}}}, Known: []Belief{k}, Tick: 10}
	st := ReadOnlyAgentState{Energy: MaxEnergy, EffectiveParanoia: ParanoiaThreshold, EffectiveCaution: CautionThreshold, SumScars: 0, BeliefCount: 1, Position: core.Position{X:0,Y:0}, Tick: 10}
	lines := Describe(obs, st)
	joined := strings.Join(lines, " ")
	if !strings.Contains(joined, "feels wrong") {
		t.Fatalf("expected hallucination cue, got: %v", lines)
	}
	if !strings.Contains(joined, "think something") {
		t.Fatalf("expected belief cue, got: %v", lines)
	}
}

func TestDescribeLowAndCriticalEnergy(t *testing.T) {
	obs := Observation{Visible: []core.TileView{}, Known: []Belief{}, Tick: 1}
	stLow := ReadOnlyAgentState{Energy: LowEnergyThreshold - 1, EffectiveParanoia: ParanoiaThreshold, EffectiveCaution: CautionThreshold, SumScars: 0, BeliefCount: 0, Position: core.Position{X:0,Y:0}, Tick: 1}
	linesLow := Describe(obs, stLow)
	if !containsAny(linesLow, []string{"sluggish"}) {
		t.Fatalf("expected low-energy cue, got: %v", linesLow)
	}
	stCrit := ReadOnlyAgentState{Energy: CriticalEnergyThreshold - 1, EffectiveParanoia: ParanoiaThreshold, EffectiveCaution: CautionThreshold, SumScars: 0, BeliefCount: 0, Position: core.Position{X:0,Y:0}, Tick: 1}
	linesCrit := Describe(obs, stCrit)
	if !containsAny(linesCrit, []string{"can't trust"}) {
		t.Fatalf("expected critical-energy cue, got: %v", linesCrit)
	}
}

func TestDescribeObserveCue(t *testing.T) {
	// Known belief at neighbor with large age to trigger observe cue
	k := Belief{Tile: core.TileView{Position: core.Position{X:1,Y:0}}, Age: CautionThreshold + 2, ScarLevel: 0}
	obs := Observation{Visible: []core.TileView{}, Known: []Belief{k}, Tick: 5}
	st := ReadOnlyAgentState{Energy: MaxEnergy, EffectiveParanoia: ParanoiaThreshold, EffectiveCaution: CautionThreshold, SumScars: 0, BeliefCount: 1, Position: core.Position{X:0,Y:0}, Tick: 5}
	lines := Describe(obs, st)
	if !containsAny(lines, []string{"steady your breathing"}) {
		t.Fatalf("expected observe cue, got: %v", lines)
	}
}

func containsAny(lines []string, subs []string) bool {
	for _, l := range lines {
		for _, s := range subs {
			if strings.Contains(l, s) { return true }
		}
	}
	return false
}
