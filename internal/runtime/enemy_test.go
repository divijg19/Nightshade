package runtime

import (
	"testing"

	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/dungeon"
	"github.com/divijg19/Nightshade/internal/world"
)

// Helper: build runtime with single remote human and let them enter dungeon
func setupRuntimeWithAgent(t *testing.T) (*Runtime, *agent.RemoteHuman) {
	t.Helper()
	a := agent.NewRemoteHumanFromExisting("A", agent.NewMemory(), agent.MaxEnergy)
	rt := New([]agent.Agent{a})
	// enter dungeon via signal
	a.RecvInput <- "ENTER_SIGNAL S000"
	rt.TickOnce()
	return rt, a
}

func TestEnemyMovesTowardPlayerDeterministically(t *testing.T) {
	rt, a := setupRuntimeWithAgent(t)
	d := rt.dungeonByAgent[a.ID()]
	if d == nil {
		t.Fatalf("expected dungeon instance")
	}
	if len(d.Entities) == 0 {
		t.Fatalf("expected at least one entity")
	}
	e0 := d.Entities[0].Pos
	// place player two tiles away so movement will occur toward player
	rt.world.SetPosition(a.ID(), world.Position{X: e0.X, Y: e0.Y + 2})
	moved := false
	// run a few ticks to catch movement cadence
	for i := 0; i < 3; i++ {
		a.RecvInput <- "."
		rt.TickOnce()
		if d.Entities[0].Pos != e0 {
			moved = true
			break
		}
	}
	if !moved {
		t.Fatalf("expected entity to move from %v, stayed same", e0)
	}
}

func TestEnemyVisibleOnlyWithinVision(t *testing.T) {
	rt, a := setupRuntimeWithAgent(t)
	d := rt.dungeonByAgent[a.ID()]
	e := d.Entities[0]
	// place player adjacent to entity to make it visible
	rt.world.SetPosition(a.ID(), world.Position{X: e.Pos.X, Y: e.Pos.Y + 1})
	snap := rt.snapshotFor(a, agent.Action(-1))
	if len(snap.Dungeon.Enemies) == 0 {
		t.Fatalf("expected enemy visible when within vision")
	}
}

func TestEnemyDamageAndForcedEjectOnCollapse(t *testing.T) {
	rt, a := setupRuntimeWithAgent(t)
	d := rt.dungeonByAgent[a.ID()]
	e := d.Entities[0]
	// set player adjacent and low energy
	rt.world.SetPosition(a.ID(), world.Position{X: e.Pos.X, Y: e.Pos.Y + 1})
	a.AdjustEnergy(-(agent.MaxEnergy - 1)) // set energy to 1
	a.RecvInput <- "."
	rt.TickOnce()
	// after entity effect, agent should be ejected (binding removed)
	if _, ok := rt.dungeonByAgent[a.ID()]; ok {
		t.Fatalf("expected agent to be ejected on collapse")
	}
}

func TestThreatLevelEscalation(t *testing.T) {
	rt, a := setupRuntimeWithAgent(t)
	d := rt.dungeonByAgent[a.ID()]
	e := d.Entities[0]
	// place player near entity and increase aggro deterministically by ticking
	rt.world.SetPosition(a.ID(), world.Position{X: e.Pos.X + 1, Y: e.Pos.Y})
	// run a few ticks to raise aggro
	for i := 0; i < 4; i++ {
		a.RecvInput <- "."
		rt.TickOnce()
	}
	snap := rt.snapshotFor(a, agent.Action(-1))
	if snap.Dungeon.Threat == "LOW" {
		t.Fatalf("expected threat to escalate above LOW, got %s", snap.Dungeon.Threat)
	}
}

func TestEnemyHaltsAfterEject(t *testing.T) {
	rt, a := setupRuntimeWithAgent(t)
	d := rt.dungeonByAgent[a.ID()]
	_ = d.Entities[0]
	// cause collapse eject
	epos0 := d.Entities[0].Pos
	rt.world.SetPosition(a.ID(), world.Position{X: epos0.X, Y: epos0.Y + 1})
	a.AdjustEnergy(-(agent.MaxEnergy - 1))
	a.RecvInput <- "."
	rt.TickOnce()
	// capture entity pos after the eject tick
	posAfter := d.Entities[0].Pos
	// next tick with no agents bound (entity should not act)
	a.RecvInput <- "."
	rt.TickOnce()
	if d.Entities[0].Pos != posAfter {
		t.Fatalf("expected entity to not move after eject; moved from %v to %v", posAfter, d.Entities[0].Pos)
	}
}

func TestMultiplayerDeterministicTargeting(t *testing.T) {
	// create two agents bound to same dungeon instance
	a := agent.NewRemoteHumanFromExisting("A", agent.NewMemory(), agent.MaxEnergy)
	b := agent.NewRemoteHumanFromExisting("B", agent.NewMemory(), agent.MaxEnergy)
	rt := New([]agent.Agent{a, b})
	// create shared instance and bind both agents directly for test
	inst := dungeon.NewInstance("D-test", dungeon.AnchorMemoryVault)
	inst.AddDefaultEntities()
	rt.dungeonByAgent[a.ID()] = inst
	rt.dungeonByAgent[b.ID()] = inst
	// place A closer to entity than B
	e := inst.Entities[0]
	rt.world.SetPosition(a.ID(), world.Position{X: e.Pos.X, Y: e.Pos.Y + 1})
	rt.world.SetPosition(b.ID(), world.Position{X: e.Pos.X, Y: e.Pos.Y + 3})
	a.RecvInput <- "."
	b.RecvInput <- "."
	rt.TickOnce()
	// ensure entity moved closer to A than to B
	epos := inst.Entities[0].Pos
	posA, _ := rt.world.PositionOf(a.ID())
	posB, _ := rt.world.PositionOf(b.ID())
	da := epos.X - posA.X
	if da < 0 {
		da = -da
	}
	daY := epos.Y - posA.Y
	if daY < 0 {
		daY = -daY
	}
	db := epos.X - posB.X
	if db < 0 {
		db = -db
	}
	dbY := epos.Y - posB.Y
	if dbY < 0 {
		dbY = -dbY
	}
	distA := da + daY
	distB := db + dbY
	if distA >= distB {
		t.Fatalf("expected entity to prefer closer agent A (distA=%d distB=%d)", distA, distB)
	}
}
