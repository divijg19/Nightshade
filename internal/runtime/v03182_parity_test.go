package runtime

import (
	"testing"

	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/world"
)

func TestBoardModeMovementDoesNotMutateRuntimeState(t *testing.T) {
	a := agent.NewRemoteHumanFromExisting("A", agent.NewMemory(), agent.MaxEnergy)
	rt := New([]agent.Agent{a})

	a.RecvInput <- "w"
	rt.TickOnce()
	obs := mustRecvObs(t, a)
	if obs.Mode != "board" {
		t.Fatalf("expected board mode on movement key outside dungeon, got %q", obs.Mode)
	}
	if obs.Dungeon != nil {
		t.Fatalf("expected no dungeon payload in board mode")
	}
	if _, inDungeon := rt.dungeonByAgent[a.ID()]; inDungeon {
		t.Fatalf("expected no dungeon binding in board mode")
	}
}

func TestBoardTicksDoNotMutateDungeonPressure(t *testing.T) {
	a := agent.NewRemoteHumanFromExisting("A", agent.NewMemory(), agent.MaxEnergy)
	rt := New([]agent.Agent{a})

	for i := 0; i < 5; i++ {
		a.RecvInput <- "."
		rt.TickOnce()
		obs := mustRecvObs(t, a)
		if obs.Mode != "board" {
			t.Fatalf("expected board mode, got %q", obs.Mode)
		}
		if obs.Dungeon != nil {
			t.Fatalf("expected nil dungeon snapshot in board mode")
		}
	}
}

func TestExitClearsBindingAndStopsPressureMutation(t *testing.T) {
	a := agent.NewRemoteHumanFromExisting("A", agent.NewMemory(), agent.MaxEnergy)
	rt := New([]agent.Agent{a})

	a.RecvInput <- "ENTER_SIGNAL S000"
	rt.TickOnce()
	_ = mustRecvObs(t, a)

	d := rt.dungeonByAgent[a.ID()]
	d.Entities = nil
	d.ObjectiveCompleted = true
	rt.world.SetPosition(a.ID(), world.Position{X: d.Exit.X, Y: d.Exit.Y + 1})

	a.RecvInput <- "w"
	rt.TickOnce()
	_ = mustRecvObs(t, a)
	a.RecvInput <- "."
	rt.TickOnce()
	_ = mustRecvObs(t, a)

	if _, ok := rt.dungeonByAgent[a.ID()]; ok {
		t.Fatalf("expected binding cleared after exit")
	}

	for i := 0; i < 3; i++ {
		a.RecvInput <- "w"
		rt.TickOnce()
		obs := mustRecvObs(t, a)
		if obs.Mode != "board" {
			t.Fatalf("expected board mode after exit, got %q", obs.Mode)
		}
		if obs.Dungeon != nil {
			t.Fatalf("expected no dungeon pressure updates after exit")
		}
	}
}

func TestSnapshotModeSwitchesBoardDungeonBoard(t *testing.T) {
	a := agent.NewRemoteHumanFromExisting("A", agent.NewMemory(), agent.MaxEnergy)
	rt := New([]agent.Agent{a})

	a.RecvInput <- "."
	rt.TickOnce()
	obsBoard := mustRecvObs(t, a)
	if obsBoard.Mode != "board" {
		t.Fatalf("expected initial board mode, got %q", obsBoard.Mode)
	}

	a.RecvInput <- "ENTER_SIGNAL S000"
	rt.TickOnce()
	_ = mustRecvObs(t, a)
	a.RecvInput <- "."
	rt.TickOnce()
	obsDungeon := mustRecvObs(t, a)
	if obsDungeon.Mode != "dungeon" {
		t.Fatalf("expected dungeon mode after enter, got %q", obsDungeon.Mode)
	}

	d := rt.dungeonByAgent[a.ID()]
	d.Entities = nil
	d.ObjectiveCompleted = true
	rt.world.SetPosition(a.ID(), world.Position{X: d.Exit.X, Y: d.Exit.Y + 1})
	a.RecvInput <- "w"
	rt.TickOnce()
	_ = mustRecvObs(t, a)
	a.RecvInput <- "."
	rt.TickOnce()
	obsBoardAgain := mustRecvObs(t, a)
	if obsBoardAgain.Mode != "board" {
		t.Fatalf("expected board mode after exit, got %q", obsBoardAgain.Mode)
	}
}
