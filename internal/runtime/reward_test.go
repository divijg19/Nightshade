package runtime

import (
	"testing"

	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/dungeon"
	"github.com/divijg19/Nightshade/internal/persist"
)

func TestSurvivalBonusOnlyOnExit(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NIGHTSHADE_DIR", dir)
	aid := t.Name()
	p := persist.DefaultProgress()
	if err := persist.SaveProgress(aid, p); err != nil {
		t.Fatalf("save: %v", err)
	}
	a := agent.NewRemoteHumanFromExisting(aid, agent.NewMemory(), agent.MaxEnergy)
	rt := New([]agent.Agent{a})
	// bind instance
	inst := dungeon.NewInstance("D-r", dungeon.AnchorMemoryVault)
	inst.Pressure = 9
	rt.dungeonByAgent[a.ID()] = inst
	// simulate forced eject (collapse) path: call applyDungeonRewards with exited=false
	// capture starting fragments and apply forced-eject rewards
	start := rt.progressByAgent[a.ID()].Fragments
	rt.applyDungeonRewards(a.ID(), inst, false)
	p2 := rt.progressByAgent[a.ID()]
	// compute fragments awarded this run directly
	fragForced := CalculateFragments(inst.Pressure, inst.InstabilityBand(), false)
	if p2.Fragments != start+fragForced {
		t.Fatalf("expected fragments %d got %d", start+fragForced, p2.Fragments)
	}
	// now test exit path: award additional fragments via exited=true
	inst2 := dungeon.NewInstance("D-r2", dungeon.AnchorMemoryVault)
	inst2.Pressure = 9
	fragExit := CalculateFragments(inst2.Pressure, inst2.InstabilityBand(), true)
	// snapshot previous fragments value (p2 is a pointer into the runtime map)
	prev := p2.Fragments
	rt.applyDungeonRewards(a.ID(), inst2, true)
	p3 := rt.progressByAgent[a.ID()]
	if p3.Fragments != prev+fragExit {
		t.Fatalf("expected fragments after exit %d got %d", prev+fragExit, p3.Fragments)
	}
}
