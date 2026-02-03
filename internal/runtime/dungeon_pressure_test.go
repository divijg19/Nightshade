package runtime

import (
	"testing"

	"github.com/divijg19/Nightshade/internal/agent"
)

func tickWithInput(t *testing.T, rt *Runtime, rh *agent.RemoteHuman, in string) agent.Observation {
	t.Helper()
	rh.RecvInput <- in
	rt.TickOnce()
	return mustRecvObs(t, rh)
}

func TestDungeonPressure_StartsAtZero(t *testing.T) {
	a := agent.NewRemoteHumanFromExisting("A", agent.NewMemory(), agent.MaxEnergy)
	rt := New([]agent.Agent{a})

	// Enter creates the dungeon during the input phase; pressure should start at 0.
	_ = tickWithInput(t, rt, a, "ENTER_SIGNAL S000")

	d := rt.dungeonByAgent["A"]
	if d == nil {
		t.Fatalf("expected dungeon instance")
	}
	if d.Pressure != 0 {
		t.Fatalf("expected pressure=0 at entry, got %d", d.Pressure)
	}
	if d.MaxPressure != 20 {
		t.Fatalf("expected maxPressure=20, got %d", d.MaxPressure)
	}
}

func TestDungeonPressure_IncrementsPerTick(t *testing.T) {
	a := agent.NewRemoteHumanFromExisting("A", agent.NewMemory(), agent.MaxEnergy)
	rt := New([]agent.Agent{a})

	_ = tickWithInput(t, rt, a, "ENTER_SIGNAL S000")

	const n = 5
	for i := 1; i <= n; i++ {
		obs := tickWithInput(t, rt, a, ".")
		if obs.Mode != "dungeon" || obs.Dungeon == nil {
			t.Fatalf("expected dungeon observation")
		}
		if obs.Dungeon.Pressure != i {
			t.Fatalf("expected snapshot pressure=%d, got %d", i, obs.Dungeon.Pressure)
		}
		if rt.dungeonByAgent["A"].Pressure != i {
			t.Fatalf("expected runtime pressure=%d, got %d", i, rt.dungeonByAgent["A"].Pressure)
		}
	}
}

func TestDungeonPressure_DoesNotIncrementOutsideDungeon(t *testing.T) {
	a := agent.NewRemoteHumanFromExisting("A", agent.NewMemory(), agent.MaxEnergy)
	rt := New([]agent.Agent{a})

	_ = tickWithInput(t, rt, a, "ENTER_SIGNAL S000")
	_ = tickWithInput(t, rt, a, ".") // pressure=1

	d := rt.dungeonByAgent["A"]
	if d == nil {
		t.Fatalf("expected dungeon")
	}
	before := d.Pressure

	// Simulate agent leaving the dungeon without implementing exit mechanics.
	delete(rt.dungeonByAgent, "A")

	_ = tickWithInput(t, rt, a, ".")
	if d.Pressure != before {
		t.Fatalf("expected pressure unchanged outside dungeon: before=%d after=%d", before, d.Pressure)
	}
}

func TestDungeonPressure_ResetsOnNewDungeon(t *testing.T) {
	a := agent.NewRemoteHumanFromExisting("A", agent.NewMemory(), agent.MaxEnergy)
	rt := New([]agent.Agent{a})

	_ = tickWithInput(t, rt, a, "ENTER_SIGNAL S000")
	_ = tickWithInput(t, rt, a, ".")
	if rt.dungeonByAgent["A"].Pressure == 0 {
		t.Fatalf("expected pressure to have incremented inside dungeon")
	}

	// New dungeon in a fresh runtime must start at 0 (no exit mechanic needed).
	b := agent.NewRemoteHumanFromExisting("B", agent.NewMemory(), agent.MaxEnergy)
	rt2 := New([]agent.Agent{b})
	_ = tickWithInput(t, rt2, b, "ENTER_SIGNAL S000")
	if rt2.dungeonByAgent["B"].Pressure != 0 {
		t.Fatalf("expected pressure reset to 0 on new dungeon, got %d", rt2.dungeonByAgent["B"].Pressure)
	}
}

func TestDungeonPressure_IsolatedPerAgentAndSnapshotMatchesRuntime(t *testing.T) {
	a := agent.NewRemoteHumanFromExisting("A", agent.NewMemory(), agent.MaxEnergy)
	b := agent.NewRemoteHumanFromExisting("B", agent.NewMemory(), agent.MaxEnergy)
	rt := New([]agent.Agent{a, b})

	// Tick 0: A enters, B waits.
	a.RecvInput <- "ENTER_SIGNAL S000"
	b.RecvInput <- "."
	rt.TickOnce()
	_ = mustRecvObs(t, a)
	_ = mustRecvObs(t, b)

	// Tick 1: B enters, A waits.
	a.RecvInput <- "."
	b.RecvInput <- "ENTER_SIGNAL S001"
	rt.TickOnce()
	_ = mustRecvObs(t, a)
	_ = mustRecvObs(t, b)

	// Tick 2: both are inside; pressure should advance independently.
	a.RecvInput <- "."
	b.RecvInput <- "."
	rt.TickOnce()
	obsA := mustRecvObs(t, a)
	obsB := mustRecvObs(t, b)

	if obsA.Mode != "dungeon" || obsA.Dungeon == nil {
		t.Fatalf("expected A dungeon snapshot")
	}
	if obsB.Mode != "dungeon" || obsB.Dungeon == nil {
		t.Fatalf("expected B dungeon snapshot")
	}

	// A has spent one more pressure-tick inside than B.
	if obsA.Dungeon.Pressure != obsB.Dungeon.Pressure+1 {
		t.Fatalf("expected A pressure = B+1, got A=%d B=%d", obsA.Dungeon.Pressure, obsB.Dungeon.Pressure)
	}

	// Snapshot must match runtime state exactly (no off-by-one).
	if obsA.Dungeon.Pressure != rt.dungeonByAgent["A"].Pressure {
		t.Fatalf("A snapshot pressure mismatch: snap=%d runtime=%d", obsA.Dungeon.Pressure, rt.dungeonByAgent["A"].Pressure)
	}
	if obsB.Dungeon.Pressure != rt.dungeonByAgent["B"].Pressure {
		t.Fatalf("B snapshot pressure mismatch: snap=%d runtime=%d", obsB.Dungeon.Pressure, rt.dungeonByAgent["B"].Pressure)
	}
}

func TestDungeonSnapshotIntegrity_InstabilityBandDungeonOnly(t *testing.T) {
	a := agent.NewRemoteHumanFromExisting("A", agent.NewMemory(), agent.MaxEnergy)
	rt := New([]agent.Agent{a})

	// Board mode snapshot: dungeon view must not be attached.
	obs0 := tickWithInput(t, rt, a, ".")
	if obs0.Mode != "board" {
		t.Fatalf("expected board mode, got %q", obs0.Mode)
	}
	if obs0.Dungeon != nil {
		t.Fatalf("expected dungeon view omitted in board mode")
	}

	// Enter dungeon.
	_ = tickWithInput(t, rt, a, "ENTER_SIGNAL S000")
	obs := tickWithInput(t, rt, a, ".")
	if obs.Mode != "dungeon" || obs.Dungeon == nil {
		t.Fatalf("expected dungeon mode with dungeon view")
	}
	if obs.Dungeon.InstabilityBand != rt.dungeonByAgent["A"].InstabilityBand() {
		t.Fatalf("instability band mismatch: snap=%d runtime=%d", obs.Dungeon.InstabilityBand, rt.dungeonByAgent["A"].InstabilityBand())
	}
}
