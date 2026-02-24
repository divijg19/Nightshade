package runtime

import (
	"testing"

	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/world"
)

func TestHideClearsTargetForOneTick(t *testing.T) {
	rt, a := setupRuntimeWithAgent(t)
	d := rt.dungeonByAgent[a.ID()]
	if len(d.Entities) == 0 {
		t.Fatal("no entities")
	}
	e := d.Entities[0]
	// place player so enemy will lock on
	rt.world.SetPosition(a.ID(), world.Position{X: e.Pos.X, Y: e.Pos.Y + 1})
	// advance one tick so enemy perceives
	a.RecvInput <- "."
	rt.TickOnce()
	// perform HIDE
	a.RecvInput <- "h"
	rt.TickOnce()
	// next tick, entity should have been cleared (unknown) for exactly one tick
	a.RecvInput <- "."
	rt.TickOnce()
	snap := rt.snapshotFor(a, agent.Action(-1))
	// any visible enemy should have Target == "unknown"
	for _, ev := range snap.Dungeon.Enemies {
		if ev.Target != "unknown" {
			t.Fatalf("expected unknown target after HIDE, got %s", ev.Target)
		}
	}
	// following tick, target should resume
	a.RecvInput <- "."
	rt.TickOnce()
	snap2 := rt.snapshotFor(a, agent.Action(-1))
	for _, ev := range snap2.Dungeon.Enemies {
		if ev.Target == "unknown" {
			t.Fatalf("expected target to resume after 1 tick, still unknown")
		}
	}
}

func TestDistractRetargetsForTwoTicks(t *testing.T) {
	rt, a := setupRuntimeWithAgent(t)
	d := rt.dungeonByAgent[a.ID()]
	if len(d.Entities) == 0 {
		t.Fatal("no entities")
	}
	e := d.Entities[0]
	// place player near entity
	rt.world.SetPosition(a.ID(), world.Position{X: e.Pos.X + 2, Y: e.Pos.Y})
	// perform DISTRACT
	a.RecvInput <- "o"
	rt.TickOnce()
	// next two ticks, the nearest enemy should target anchor or exit (not player)
	for i := 0; i < 2; i++ {
		a.RecvInput <- "."
		rt.TickOnce()
		snap := rt.snapshotFor(a, agent.Action(-1))
		for _, ev := range snap.Dungeon.Enemies {
			if ev.Target == "player" {
				t.Fatalf("expected enemy retargeted for distract tick %d", i)
			}
		}
	}
	// third tick, resume default targeting
	a.RecvInput <- "."
	rt.TickOnce()
	snap := rt.snapshotFor(a, agent.Action(-1))
	resumed := false
	for _, ev := range snap.Dungeon.Enemies {
		if ev.Target == "player" {
			resumed = true
		}
	}
	if !resumed {
		t.Fatalf("expected enemy to resume targeting player after distract")
	}
}
