package runtime

import (
	"testing"
	"time"

	"github.com/divijg19/Nightshade/internal/agent"
)

func mustRecvObs(t *testing.T, rh *agent.RemoteHuman) agent.Observation {
	t.Helper()
	select {
	case obs := <-rh.SendObservation:
		return obs
	case <-time.After(250 * time.Millisecond):
		t.Fatalf("timed out waiting for observation")
		return agent.Observation{}
	}
}

func TestDungeonCommitment_SignalLockAndBinding(t *testing.T) {
	a := agent.NewRemoteHumanFromExisting("A", agent.NewMemory(), agent.MaxEnergy)
	rt := New([]agent.Agent{a})

	// Tick 0: enter signal.
	a.RecvInput <- "ENTER_SIGNAL S000"
	rt.TickOnce()
	_ = mustRecvObs(t, a) // drain pre-input snapshot

	if _, ok := rt.dungeonByAgent["A"]; !ok {
		t.Fatalf("expected dungeon to be created for agent")
	}
	if got := rt.signalByAgent["A"]; got != "S000" {
		t.Fatalf("expected signal binding to S000, got %q", got)
	}
	if s, ok := rt.board.Find("S000"); !ok || s.LockedBy != "A" {
		t.Fatalf("expected signal S000 to be locked by A")
	}

	// Tick 1: snapshot should now be dungeon mode and must not deliver overworld board.
	a.RecvInput <- "."
	rt.TickOnce()
	obs := mustRecvObs(t, a)
	if obs.Mode != "dungeon" {
		t.Fatalf("expected dungeon mode after enter, got %q", obs.Mode)
	}
	if obs.Board != nil {
		t.Fatalf("expected overworld board to be omitted in dungeon mode")
	}
}

func TestDungeonCommitment_ContestedSignalLock(t *testing.T) {
	a := agent.NewRemoteHumanFromExisting("A", agent.NewMemory(), agent.MaxEnergy)
	b := agent.NewRemoteHumanFromExisting("B", agent.NewMemory(), agent.MaxEnergy)
	rt := New([]agent.Agent{a, b})

	// Both attempt the same signal on the same tick.
	a.RecvInput <- "ENTER_SIGNAL S000"
	b.RecvInput <- "ENTER_SIGNAL S000"
	rt.TickOnce()
	_ = mustRecvObs(t, a)
	_ = mustRecvObs(t, b)

	// Exactly one should have been bound.
	_, aIn := rt.dungeonByAgent["A"]
	_, bIn := rt.dungeonByAgent["B"]
	if aIn == bIn {
		t.Fatalf("expected exactly one agent to enter dungeon, got A=%t B=%t", aIn, bIn)
	}
}

func TestDungeonCommitment_ReentryProtection(t *testing.T) {
	a := agent.NewRemoteHumanFromExisting("A", agent.NewMemory(), agent.MaxEnergy)
	rt := New([]agent.Agent{a})

	// Enter first signal.
	a.RecvInput <- "ENTER_SIGNAL S000"
	rt.TickOnce()
	_ = mustRecvObs(t, a)

	// Attempt to enter another while inside.
	a.RecvInput <- "ENTER_SIGNAL S001"
	rt.TickOnce()
	_ = mustRecvObs(t, a)

	if got := rt.signalByAgent["A"]; got != "S000" {
		t.Fatalf("expected agent to remain bound to S000, got %q", got)
	}
	if s, ok := rt.board.Find("S001"); ok {
		if s.LockedBy != "" {
			t.Fatalf("expected S001 to remain unlocked, lockedBy=%q", s.LockedBy)
		}
	}
}
