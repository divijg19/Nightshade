package runtime

import (
	"testing"

	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/persist"
)

func TestFullProgressionCycle(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NIGHTSHADE_DIR", dir)
	aid := "REG"
	p := persist.DefaultProgress()
	if err := persist.SaveProgress(aid, p); err != nil {
		t.Fatalf("save: %v", err)
	}
	a := agent.NewRemoteHumanFromExisting(aid, agent.NewMemory(), agent.MaxEnergy)
	rt := New([]agent.Agent{a})
	// enter dungeon and simulate a few ticks to earn fragments via exit
	a.RecvInput <- "ENTER_SIGNAL S000"
	rt.TickOnce()
	// move agent to exit repeatedly and force exit path
	// simple approach: call applyDungeonRewards directly to simulate exit
	if inst, ok := rt.dungeonByAgent[a.ID()]; ok {
		rt.applyDungeonRewards(a.ID(), inst, true)
	}
	// unlock a skill via UnlockSkill + Save
	pp := rt.progressByAgent[a.ID()]
	pp.Fragments += 10
	pp.SkillPoints = pp.Fragments / 10
	if err := agent.UnlockSkill(pp, "pressure_sense"); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	if err := persist.SaveProgress(a.ID(), pp); err != nil {
		t.Fatalf("save after unlock: %v", err)
	}
	// restart runtime
	a2 := agent.NewRemoteHumanFromExisting(aid, agent.NewMemory(), agent.MaxEnergy)
	rt2 := New([]agent.Agent{a2})
	if p2, ok := rt2.progressByAgent[aid]; !ok || p2 == nil {
		t.Fatalf("expected progress after restart")
	} else {
		if !p2.UnlockedSkills["pressure_sense"] {
			t.Fatalf("expected unlocked skill to persist after restart")
		}
	}
	_ = rt2
}
