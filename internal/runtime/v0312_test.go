package runtime

import (
	"reflect"
	"testing"

	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/core"
	"github.com/divijg19/Nightshade/internal/dungeon"
	"github.com/divijg19/Nightshade/internal/world"
)

func setupV0312(t *testing.T, pathChoice string) (*Runtime, *agent.RemoteHuman) {
	t.Helper()
	a := agent.NewRemoteHumanFromExisting("A", agent.NewMemory(), agent.MaxEnergy)
	rt := New([]agent.Agent{a})
	a.RecvInput <- "ENTER_SIGNAL S000"
	rt.TickOnce()
	_ = mustRecvObs(t, a)
	a.RecvInput <- pathChoice
	rt.TickOnce()
	_ = mustRecvObs(t, a)
	return rt, a
}

func TestV0312_PathThresholdsAndChannelTiming(t *testing.T) {
	rtS, aS := setupV0312(t, "1")
	dS := rtS.dungeonByAgent[aS.ID()]
	if dS.EnrageThreshold() <= 15 {
		t.Fatalf("expected stabilizer threshold above base, got %d", dS.EnrageThreshold())
	}
	dS.Pressure = dS.EnrageThreshold()
	dS.ObjectiveCompleted = true
	rtS.world.SetPosition(aS.ID(), world.Position{X: dS.Exit.X, Y: dS.Exit.Y + 1})
	aS.RecvInput <- "w"
	rtS.TickOnce()
	if _, ok := rtS.dungeonByAgent[aS.ID()]; ok {
		t.Fatalf("expected stabilizer immediate enraged exit completion")
	}

	rtH, aH := setupV0312(t, "2")
	dH := rtH.dungeonByAgent[aH.ID()]
	dH.Pressure = dH.EnrageThreshold()
	dH.ObjectiveCompleted = true
	rtH.world.SetPosition(aH.ID(), world.Position{X: dH.Exit.X, Y: dH.Exit.Y + 1})
	aH.RecvInput <- "w"
	rtH.TickOnce()
	if _, ok := rtH.dungeonByAgent[aH.ID()]; !ok {
		t.Fatalf("expected non-stabilizer to remain bound during channel")
	}
}

func TestV0312_PathPressureDeltaAndDeterminism(t *testing.T) {
	rtS, aS := setupV0312(t, "1")
	rtH, aH := setupV0312(t, "2")
	dS := rtS.dungeonByAgent[aS.ID()]
	dH := rtH.dungeonByAgent[aH.ID()]

	for i := 0; i < 4; i++ {
		aS.RecvInput <- "."
		rtS.TickOnce()
		_ = mustRecvObs(t, aS)
		aH.RecvInput <- "."
		rtH.TickOnce()
		_ = mustRecvObs(t, aH)
	}
	if dH.Pressure <= dS.Pressure {
		t.Fatalf("expected harvester pressure to exceed stabilizer pressure: H=%d S=%d", dH.Pressure, dS.Pressure)
	}

	rt1, a1 := setupV0312(t, "3")
	rt2, a2 := setupV0312(t, "3")
	timeline1 := []int{}
	timeline2 := []int{}
	for i := 0; i < 6; i++ {
		a1.RecvInput <- "."
		rt1.TickOnce()
		_ = mustRecvObs(t, a1)
		timeline1 = append(timeline1, rt1.dungeonByAgent[a1.ID()].Pressure)
		a2.RecvInput <- "."
		rt2.TickOnce()
		_ = mustRecvObs(t, a2)
		timeline2 = append(timeline2, rt2.dungeonByAgent[a2.ID()].Pressure)
	}
	if !reflect.DeepEqual(timeline1, timeline2) {
		t.Fatalf("expected deterministic pressure timeline, got %v and %v", timeline1, timeline2)
	}
}

func TestV0312_MutatorStabilityAndFragileTransition(t *testing.T) {
	rt1, a1 := setupV0312(t, "1")
	rt2, a2 := setupV0312(t, "1")
	d1 := rt1.dungeonByAgent[a1.ID()]
	d2 := rt2.dungeonByAgent[a2.ID()]
	if len(d1.MutatorTiles) != len(d2.MutatorTiles) {
		t.Fatalf("expected deterministic mutator count")
	}
	for i := range d1.MutatorTiles {
		if d1.MutatorTiles[i].Pos != d2.MutatorTiles[i].Pos || d1.MutatorTiles[i].Type != d2.MutatorTiles[i].Type {
			t.Fatalf("expected stable mutator layout")
		}
	}

	idx := -1
	for i := range d1.MutatorTiles {
		if d1.MutatorTiles[i].Type == dungeon.MutatorFragileFloor {
			idx = i
			break
		}
	}
	if idx == -1 {
		t.Skip("no fragile mutator seeded for this deterministic id")
	}
	frag := d1.MutatorTiles[idx].Pos
	rt1.world.SetPosition(a1.ID(), world.Position{X: frag.X, Y: frag.Y})
	a1.RecvInput <- "."
	rt1.TickOnce()
	_ = mustRecvObs(t, a1)
	a1.RecvInput <- "."
	rt1.TickOnce()
	_ = mustRecvObs(t, a1)
	if d1.MutatorTiles[idx].Type != dungeon.MutatorCorruptionZone {
		t.Fatalf("expected fragile floor to transition into corruption zone")
	}
}

func TestV0312_PhaseTransitionAndNestSpawn(t *testing.T) {
	rt, a := setupV0312(t, "3")
	d := rt.dungeonByAgent[a.ID()]
	startEntities := len(d.Entities)
	rt.world.SetPosition(a.ID(), world.Position{X: d.Anchor.X, Y: d.Anchor.Y + 1})
	a.RecvInput <- "w"
	rt.TickOnce()
	_ = mustRecvObs(t, a)
	if d.RunPhase != "II" {
		t.Fatalf("expected phase II after reaching anchor, got %s", d.RunPhase)
	}
	if len(d.Entities) < startEntities {
		t.Fatalf("expected deterministic nest spawn to not reduce entities")
	}
	rt.world.SetPosition(a.ID(), world.Position{X: d.Anchor.X, Y: d.Anchor.Y})
	a.RecvInput <- "e"
	rt.TickOnce()
	_ = mustRecvObs(t, a)
	if d.RunPhase != "III" {
		t.Fatalf("expected phase III after core interaction, got %s", d.RunPhase)
	}
}

func TestV0312_AbilityCooldownAndSnapshot(t *testing.T) {
	rt, a := setupV0312(t, "1")
	d := rt.dungeonByAgent[a.ID()]
	a.RecvInput <- "f"
	rt.TickOnce()
	_ = mustRecvObs(t, a)
	if d.AbilityCooldown != 5 {
		t.Fatalf("expected cooldown set to 5, got %d", d.AbilityCooldown)
	}
	if d.SuppressTicks != 2 {
		t.Fatalf("expected suppress effect for 2 ticks")
	}
	a.RecvInput <- "f"
	rt.TickOnce()
	obs := mustRecvObs(t, a)
	if d.AbilityCooldown >= 5 {
		t.Fatalf("expected cooldown to decrement over ticks")
	}
	if obs.Dungeon == nil || obs.Dungeon.AbilityCooldown <= 0 {
		t.Fatalf("expected ability cooldown in snapshot")
	}
	if obs.Dungeon.PathType == "" {
		t.Fatalf("expected path type in snapshot")
	}
	if len(obs.Dungeon.MutatorTiles) == 0 {
		t.Fatalf("expected mutator metadata in snapshot")
	}
}

func TestV0312_BoardBiasInSnapshot(t *testing.T) {
	a := agent.NewRemoteHumanFromExisting("A", agent.NewMemory(), agent.MaxEnergy)
	rt := New([]agent.Agent{a})
	a.RecvInput <- "."
	rt.TickOnce()
	obs := mustRecvObs(t, a)
	if obs.Board == nil || len(obs.Board.Signals) == 0 {
		t.Fatalf("expected board signals")
	}
	if obs.Board.Signals[0].SignalBias == "" {
		t.Fatalf("expected signal bias in board snapshot")
	}
}

func TestV0312_MutatorLayoutSameSignalTwice(t *testing.T) {
	makeLayout := func() []core.Position {
		a := agent.NewRemoteHumanFromExisting("A", agent.NewMemory(), agent.MaxEnergy)
		rt := New([]agent.Agent{a})
		a.RecvInput <- "ENTER_SIGNAL S000"
		rt.TickOnce()
		_ = mustRecvObs(t, a)
		d := rt.dungeonByAgent[a.ID()]
		out := make([]core.Position, 0, len(d.MutatorTiles))
		for _, mt := range d.MutatorTiles {
			out = append(out, mt.Pos)
		}
		return out
	}
	if !reflect.DeepEqual(makeLayout(), makeLayout()) {
		t.Fatalf("expected same signal to produce identical mutator layout")
	}
}
