package runtime

import (
	"testing"

	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/world"
)

func setupSummaryRT(t *testing.T) (*Runtime, *agent.RemoteHuman) {
	t.Helper()
	a := agent.NewRemoteHumanFromExisting("A", agent.NewMemory(), agent.MaxEnergy)
	rt := New([]agent.Agent{a})
	a.RecvInput <- "ENTER_SIGNAL S000"
	rt.TickOnce()
	_ = mustRecvObs(t, a)
	return rt, a
}

func TestRunSummaryAppearsExactlyOnceAndWaitsForEnter(t *testing.T) {
	rt, a := setupSummaryRT(t)
	d := rt.dungeonByAgent[a.ID()]
	d.Entities = nil
	d.ObjectiveCompleted = true
	rt.world.SetPosition(a.ID(), world.Position{X: d.Exit.X, Y: d.Exit.Y + 1})

	a.RecvInput <- "w"
	rt.TickOnce()
	_ = mustRecvObs(t, a)
	a.RecvInput <- "."
	rt.TickOnce()
	obs1 := mustRecvObs(t, a)
	if obs1.RunSummary == nil {
		t.Fatalf("expected run summary on first board frame")
	}

	// summary should not repeat on subsequent frames while awaiting enter
	a.RecvInput <- "w"
	rt.TickOnce()
	obs2 := mustRecvObs(t, a)
	if obs2.RunSummary != nil {
		t.Fatalf("expected run summary exactly once")
	}

	// Enter clears pending summary state
	a.RecvInput <- "."
	rt.TickOnce()
	obs3 := mustRecvObs(t, a)
	if obs3.RunSummary != nil {
		t.Fatalf("expected summary cleared after Enter")
	}
}

func TestRunSummaryFieldsMatchRuntime(t *testing.T) {
	rt, a := setupSummaryRT(t)
	d := rt.dungeonByAgent[a.ID()]
	d.Entities = nil
	d.ObjectiveCompleted = true
	d.PeakPressure = 17
	d.TimeInSignal = 18
	rt.world.SetPosition(a.ID(), world.Position{X: d.Exit.X, Y: d.Exit.Y + 1})

	a.RecvInput <- "w"
	rt.TickOnce()
	_ = mustRecvObs(t, a)
	a.RecvInput <- "."
	rt.TickOnce()
	obs := mustRecvObs(t, a)
	if obs.RunSummary == nil {
		t.Fatalf("expected run summary")
	}
	if obs.RunSummary.ResultType != "stabilized" {
		t.Fatalf("expected result stabilized, got %q", obs.RunSummary.ResultType)
	}
	if obs.RunSummary.PeakPressure != 17 {
		t.Fatalf("expected peak pressure 17, got %d", obs.RunSummary.PeakPressure)
	}
	if obs.RunSummary.TimeInSignal != 19 {
		t.Fatalf("expected time in signal 19, got %d", obs.RunSummary.TimeInSignal)
	}
}

func TestExitChannelReducedTickCount(t *testing.T) {
	rt, a := setupSummaryRT(t)
	d := rt.dungeonByAgent[a.ID()]
	d.Pressure = 15
	d.Entities = nil
	d.ObjectiveCompleted = true
	rt.world.SetPosition(a.ID(), world.Position{X: d.Exit.X, Y: d.Exit.Y + 1})

	a.RecvInput <- "w"
	rt.TickOnce()
	if !d.ExitChanneling {
		t.Fatalf("expected channel to start")
	}

	a.RecvInput <- "."
	rt.TickOnce()
	_ = mustRecvObs(t, a)
	a.RecvInput <- "."
	rt.TickOnce()
	if _, ok := rt.dungeonByAgent[a.ID()]; ok {
		t.Fatalf("expected exit completion with reduced channel duration")
	}
}

func TestOneLineEventPriorityRuntime(t *testing.T) {
	rt, a := setupSummaryRT(t)
	rt.setPendingEvent(a.ID(), "→ Move NORTH (-1)")
	rt.setPendingEvent(a.ID(), "Instability surges (+2).")
	rt.setPendingEvent(a.ID(), "The signal shatters.")
	s := rt.snapshotFor(a, agent.Action(-1))
	if s.Event != "The signal shatters." {
		t.Fatalf("expected highest-priority event only, got %q", s.Event)
	}
}

func TestRunSummaryPendingDoesNotBlockEnterSignal(t *testing.T) {
	rt, a := setupSummaryRT(t)
	d := rt.dungeonByAgent[a.ID()]
	d.Entities = nil
	d.ObjectiveCompleted = true
	rt.world.SetPosition(a.ID(), world.Position{X: d.Exit.X, Y: d.Exit.Y + 1})

	a.RecvInput <- "w"
	rt.TickOnce()
	_ = mustRecvObs(t, a)
	a.RecvInput <- "."
	rt.TickOnce()
	obsSummary := mustRecvObs(t, a)
	if obsSummary.RunSummary == nil {
		t.Fatalf("expected run summary frame before re-entry")
	}

	a.RecvInput <- "ENTER_SIGNAL S001"
	rt.TickOnce()
	_ = mustRecvObs(t, a)

	a.RecvInput <- "."
	rt.TickOnce()
	obsDungeon := mustRecvObs(t, a)
	if obsDungeon.Mode != "dungeon" {
		t.Fatalf("expected dungeon mode after enter while summary was pending, got %q", obsDungeon.Mode)
	}
}
