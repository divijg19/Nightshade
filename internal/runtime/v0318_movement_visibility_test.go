package runtime

import (
	"testing"

	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/world"
)

func enterDungeonAndSnapshot(t *testing.T, rt *Runtime, a *agent.RemoteHuman) agent.Observation {
	t.Helper()
	a.RecvInput <- "ENTER_SIGNAL S000"
	rt.TickOnce()
	_ = mustRecvObs(t, a)

	a.RecvInput <- "."
	rt.TickOnce()
	obs := mustRecvObs(t, a)
	if obs.Mode != "dungeon" || obs.Dungeon == nil {
		t.Fatalf("expected dungeon observation, got mode=%q", obs.Mode)
	}
	if !obs.Dungeon.PlayerPosKnown {
		t.Fatalf("expected dungeon snapshot to include player coordinates")
	}
	return obs
}

func applyInputAndGetPostActionSnapshot(t *testing.T, rt *Runtime, a *agent.RemoteHuman, input string) agent.Observation {
	t.Helper()
	a.RecvInput <- input
	rt.TickOnce()
	_ = mustRecvObs(t, a)

	a.RecvInput <- "."
	rt.TickOnce()
	obs := mustRecvObs(t, a)
	if obs.Mode != "dungeon" || obs.Dungeon == nil {
		t.Fatalf("expected dungeon observation, got mode=%q", obs.Mode)
	}
	return obs
}

func TestDungeonMovementUpdatesSnapshotPlayerPosition(t *testing.T) {
	a := agent.NewRemoteHumanFromExisting("A", agent.NewMemory(), agent.MaxEnergy)
	rt := New([]agent.Agent{a})

	obs0 := enterDungeonAndSnapshot(t, rt, a)
	x0, y0 := obs0.Dungeon.PlayerX, obs0.Dungeon.PlayerY

	obsEast := applyInputAndGetPostActionSnapshot(t, rt, a, "d")
	if obsEast.Dungeon.PlayerX != x0+1 || obsEast.Dungeon.PlayerY != y0 {
		t.Fatalf("expected east move to update player to (%d,%d), got (%d,%d)", x0+1, y0, obsEast.Dungeon.PlayerX, obsEast.Dungeon.PlayerY)
	}
	if obsEast.Position.X != obsEast.Dungeon.PlayerX || obsEast.Position.Y != obsEast.Dungeon.PlayerY {
		t.Fatalf("expected snapshot position parity: obs=%+v dungeon=(%d,%d)", obsEast.Position, obsEast.Dungeon.PlayerX, obsEast.Dungeon.PlayerY)
	}

	obsWest := applyInputAndGetPostActionSnapshot(t, rt, a, "a")
	if obsWest.Dungeon.PlayerX != x0 || obsWest.Dungeon.PlayerY != y0 {
		t.Fatalf("expected west move to return player to (%d,%d), got (%d,%d)", x0, y0, obsWest.Dungeon.PlayerX, obsWest.Dungeon.PlayerY)
	}

	obsNorth := applyInputAndGetPostActionSnapshot(t, rt, a, "w")
	if obsNorth.Dungeon.PlayerY != y0-1 {
		t.Fatalf("expected north move to decrement Y by 1, before=%d after=%d", y0, obsNorth.Dungeon.PlayerY)
	}
	if obsNorth.Dungeon.PlayerY < 0 || obsNorth.Dungeon.PlayerY >= len(obsNorth.Dungeon.Grid) {
		t.Fatalf("player Y out of bounds after move: %d", obsNorth.Dungeon.PlayerY)
	}
	if obsNorth.Dungeon.PlayerX < 0 || obsNorth.Dungeon.PlayerX >= len(obsNorth.Dungeon.Grid[obsNorth.Dungeon.PlayerY]) {
		t.Fatalf("player X out of bounds after move: %d", obsNorth.Dungeon.PlayerX)
	}
}

func TestDungeonMoveBlockedByWallKeepsSnapshotPosition(t *testing.T) {
	a := agent.NewRemoteHumanFromExisting("A", agent.NewMemory(), agent.MaxEnergy)
	rt := New([]agent.Agent{a})

	_ = enterDungeonAndSnapshot(t, rt, a)
	rt.world.SetPosition(a.ID(), world.Position{X: 1, Y: 1})

	a.RecvInput <- "."
	rt.TickOnce()
	before := mustRecvObs(t, a)
	if before.Dungeon == nil {
		t.Fatalf("expected dungeon snapshot before wall test")
	}

	after := applyInputAndGetPostActionSnapshot(t, rt, a, "a")
	if after.Dungeon.PlayerX != before.Dungeon.PlayerX || after.Dungeon.PlayerY != before.Dungeon.PlayerY {
		t.Fatalf("expected blocked wall move to keep position (%d,%d), got (%d,%d)", before.Dungeon.PlayerX, before.Dungeon.PlayerY, after.Dungeon.PlayerX, after.Dungeon.PlayerY)
	}
	if after.Position != before.Position {
		t.Fatalf("expected blocked wall move to keep snapshot position: before=%+v after=%+v", before.Position, after.Position)
	}
}
