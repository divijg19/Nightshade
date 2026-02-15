package runtime

import (
	"testing"

	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/core"
	"github.com/divijg19/Nightshade/internal/dungeon"
	"github.com/divijg19/Nightshade/internal/world"
)

func TestDeterministicObjectiveAssignmentBySignalID(t *testing.T) {
	d0 := dungeon.NewInstance("D-S000", dungeon.AnchorMemoryVault)
	d1 := dungeon.NewInstance("D-S001", dungeon.AnchorMemoryVault)
	d2 := dungeon.NewInstance("D-S002", dungeon.AnchorMemoryVault)
	d3 := dungeon.NewInstance("D-S003", dungeon.AnchorMemoryVault)

	if d0.ObjectiveType != "HUNT" {
		t.Fatalf("expected HUNT for S000, got %s", d0.ObjectiveType)
	}
	if d1.ObjectiveType != "STABILIZE" {
		t.Fatalf("expected STABILIZE for S001, got %s", d1.ObjectiveType)
	}
	if d2.ObjectiveType != "PURGE" {
		t.Fatalf("expected PURGE for S002, got %s", d2.ObjectiveType)
	}
	if d3.ObjectiveType != "RETRIEVE" {
		t.Fatalf("expected RETRIEVE for S003, got %s", d3.ObjectiveType)
	}

	d0b := dungeon.NewInstance("D-S000", dungeon.AnchorMemoryVault)
	if d0.ObjectiveType != d0b.ObjectiveType {
		t.Fatalf("objective assignment changed across runs: %s vs %s", d0.ObjectiveType, d0b.ObjectiveType)
	}
}

func TestObjectiveStabilizeRequiresAnchorTicks(t *testing.T) {
	rt, a := setupRuntimeWithAgent(t)
	d := rt.dungeonByAgent[a.ID()]
	d.ObjectiveType = "STABILIZE"
	d.ObjectiveTarget = 2
	d.ObjectiveProgress = 0
	d.ObjectiveCompleted = false

	rt.world.SetPosition(a.ID(), world.Position{X: d.Anchor.X, Y: d.Anchor.Y})
	a.RecvInput <- "."
	rt.TickOnce()
	if d.ObjectiveProgress != 1 {
		t.Fatalf("expected stabilize progress 1, got %d", d.ObjectiveProgress)
	}

	rt.world.SetPosition(a.ID(), world.Position{X: d.Anchor.X + 1, Y: d.Anchor.Y})
	a.RecvInput <- "."
	rt.TickOnce()
	if d.ObjectiveProgress != 0 {
		t.Fatalf("expected stabilize progress reset off anchor, got %d", d.ObjectiveProgress)
	}

	rt.world.SetPosition(a.ID(), world.Position{X: d.Anchor.X, Y: d.Anchor.Y})
	a.RecvInput <- "."
	rt.TickOnce()
	a.RecvInput <- "."
	rt.TickOnce()
	if !d.ObjectiveCompleted {
		t.Fatalf("expected stabilize objective completed")
	}
}

func TestObjectivePurgeObserveOnNodeOnly(t *testing.T) {
	rt, a := setupRuntimeWithAgent(t)
	d := rt.dungeonByAgent[a.ID()]
	d.ObjectiveType = "PURGE"
	d.ObjectiveProgress = 0
	d.ObjectiveCompleted = false
	node := core.Position{X: 2, Y: 2}
	d.PurgeNodes = []core.Position{node}
	d.ObjectiveTarget = 1

	rt.world.SetPosition(a.ID(), world.Position{X: 1, Y: 1})
	a.RecvInput <- "e"
	rt.TickOnce()
	if d.ObjectiveProgress != 0 {
		t.Fatalf("expected no purge progress off node, got %d", d.ObjectiveProgress)
	}

	rt.world.SetPosition(a.ID(), world.Position{X: node.X, Y: node.Y})
	a.RecvInput <- "e"
	rt.TickOnce()
	if d.ObjectiveProgress != 1 || !d.ObjectiveCompleted {
		t.Fatalf("expected purge completion on node, progress=%d completed=%v", d.ObjectiveProgress, d.ObjectiveCompleted)
	}
}

func TestObjectiveRetrieveResetsWhenSteppingOff(t *testing.T) {
	rt, a := setupRuntimeWithAgent(t)
	d := rt.dungeonByAgent[a.ID()]
	d.ObjectiveType = "RETRIEVE"
	d.ObjectiveTarget = 2
	d.ObjectiveProgress = 0
	d.RetrieveHold = 0
	d.ObjectiveCompleted = false

	rt.world.SetPosition(a.ID(), world.Position{X: d.Anchor.X, Y: d.Anchor.Y})
	a.RecvInput <- "."
	rt.TickOnce()
	if d.RetrieveHold != 1 || d.ObjectiveProgress != 1 {
		t.Fatalf("expected hold/progress 1, got hold=%d progress=%d", d.RetrieveHold, d.ObjectiveProgress)
	}

	rt.world.SetPosition(a.ID(), world.Position{X: d.Anchor.X + 1, Y: d.Anchor.Y})
	a.RecvInput <- "."
	rt.TickOnce()
	if d.RetrieveHold != 0 || d.ObjectiveProgress != 0 {
		t.Fatalf("expected hold reset off vault, got hold=%d progress=%d", d.RetrieveHold, d.ObjectiveProgress)
	}

	rt.world.SetPosition(a.ID(), world.Position{X: d.Anchor.X, Y: d.Anchor.Y})
	a.RecvInput <- "."
	rt.TickOnce()
	a.RecvInput <- "."
	rt.TickOnce()
	if !d.ObjectiveCompleted {
		t.Fatalf("expected retrieve objective completed")
	}
}

func TestObjectiveHuntCompletesWhenEliteRemoved(t *testing.T) {
	rt, a := setupRuntimeWithAgent(t)
	d := rt.dungeonByAgent[a.ID()]
	d.ObjectiveType = "HUNT"
	d.ObjectiveTarget = 1
	d.ObjectiveProgress = 0
	d.ObjectiveCompleted = false
	d.Entities = []dungeon.Entity{{ID: "elite-0", Pos: core.Position{X: 2, Y: 2}, Kind: dungeon.EnemyHunter}}

	a.RecvInput <- "."
	rt.TickOnce()
	if d.ObjectiveCompleted {
		t.Fatalf("did not expect hunt complete while elite exists")
	}

	d.Entities = []dungeon.Entity{{ID: "e-1", Pos: core.Position{X: 2, Y: 2}, Kind: dungeon.EnemyHunter}}
	a.RecvInput <- "."
	rt.TickOnce()
	if !d.ObjectiveCompleted {
		t.Fatalf("expected hunt objective completed after elite removed")
	}
}

func TestExitBlockedBeforeObjectiveCompletion(t *testing.T) {
	rt, a := setupRuntimeWithAgent(t)
	d := rt.dungeonByAgent[a.ID()]
	d.ObjectiveType = "STABILIZE"
	d.ObjectiveTarget = 5
	d.ObjectiveProgress = 0
	d.ObjectiveCompleted = false
	rt.world.SetPosition(a.ID(), world.Position{X: d.Exit.X, Y: d.Exit.Y + 1})

	a.RecvInput <- "w"
	rt.TickOnce()
	if _, ok := rt.dungeonByAgent[a.ID()]; !ok {
		t.Fatalf("expected dungeon binding retained when objective incomplete")
	}
	snap := rt.snapshotFor(a, agent.Action(-1))
	if snap.Event != "Objective incomplete." {
		t.Fatalf("expected objective incomplete event, got %q", snap.Event)
	}
}

func TestExitAllowedAfterObjectiveCompletion(t *testing.T) {
	rt, a := setupRuntimeWithAgent(t)
	d := rt.dungeonByAgent[a.ID()]
	d.ObjectiveCompleted = true
	rt.world.SetPosition(a.ID(), world.Position{X: d.Exit.X, Y: d.Exit.Y + 1})

	a.RecvInput <- "w"
	rt.TickOnce()
	if _, ok := rt.dungeonByAgent[a.ID()]; ok {
		t.Fatalf("expected dungeon binding removed after successful exit")
	}
}

func TestCoreIntegrityDecaysCorrectly(t *testing.T) {
	rt, a := setupRuntimeWithAgent(t)
	d := rt.dungeonByAgent[a.ID()]
	d.CoreIntegrity = 100
	d.Pressure = 15
	d.Entities = []dungeon.Entity{{ID: "e-0", Pos: d.Anchor, Kind: dungeon.EnemyHunter}}

	a.RecvInput <- "."
	rt.TickOnce()
	if d.CoreIntegrity != 97 {
		t.Fatalf("expected integrity 97, got %d", d.CoreIntegrity)
	}
}

func TestCoreIntegrityCollapseForcesEject(t *testing.T) {
	rt, a := setupRuntimeWithAgent(t)
	d := rt.dungeonByAgent[a.ID()]
	d.CoreIntegrity = 1
	d.Pressure = 15
	d.Entities = []dungeon.Entity{{ID: "e-0", Pos: d.Anchor, Kind: dungeon.EnemyHunter}}

	a.RecvInput <- "."
	rt.TickOnce()
	if _, ok := rt.dungeonByAgent[a.ID()]; ok {
		t.Fatalf("expected dungeon binding removed on integrity collapse")
	}
	obs := mustRecvObs(t, a)
	if obs.Mode != "board" {
		t.Fatalf("expected board mode after integrity collapse, got %q", obs.Mode)
	}
}

func TestEnrageTriggersAtThreshold(t *testing.T) {
	rt, a := setupRuntimeWithAgent(t)
	d := rt.dungeonByAgent[a.ID()]
	d.ObjectiveType = "PURGE"
	d.ObjectiveTarget = 10
	d.ObjectiveProgress = 7
	d.Pressure = 14
	d.Phase = "NORMAL"
	rt.world.SetPosition(a.ID(), world.Position{X: 1, Y: 1})

	a.RecvInput <- "."
	rt.TickOnce()
	if d.Phase != "NORMAL" {
		t.Fatalf("expected NORMAL below threshold, got %s", d.Phase)
	}

	a.RecvInput <- "."
	rt.TickOnce()
	if d.Phase != "ENRAGED" {
		t.Fatalf("expected ENRAGED at threshold, got %s", d.Phase)
	}
}

func TestEnrageModifiesEnemyCadenceDeterministically(t *testing.T) {
	rt, a := setupRuntimeWithAgent(t)
	d := rt.dungeonByAgent[a.ID()]
	d.ObjectiveType = "PURGE"
	d.ObjectiveTarget = 10
	d.ObjectiveProgress = 0
	d.Pressure = 0
	d.Entities = []dungeon.Entity{{ID: "w-0", Pos: core.Position{X: 1, Y: 1}, Kind: dungeon.EnemyWarden}}
	rt.world.SetPosition(a.ID(), world.Position{X: 5, Y: 5})

	rt.tick = 1
	start := d.Entities[0].Pos
	a.RecvInput <- "."
	rt.TickOnce()
	if d.Entities[0].Pos != start {
		t.Fatalf("expected warden to hold on odd tick in NORMAL phase")
	}

	d.Entities[0].Pos = start
	d.Pressure = 16
	rt.tick = 1
	a.RecvInput <- "."
	rt.TickOnce()
	if d.Entities[0].Pos == start {
		t.Fatalf("expected warden to move on odd tick in ENRAGED phase")
	}
}

func TestRewardIncludesIntegrityBonus(t *testing.T) {
	low := CalculateFragments(9, 0, true, true, 0)
	high := CalculateFragments(9, 0, true, true, 100)
	if high <= low {
		t.Fatalf("expected higher reward with higher integrity: low=%d high=%d", low, high)
	}
	if high-low != 2 {
		t.Fatalf("expected integrity bonus delta 2, got %d", high-low)
	}
}

func TestMultiplayerObjectiveIsolation(t *testing.T) {
	a := agent.NewRemoteHumanFromExisting("A", agent.NewMemory(), agent.MaxEnergy)
	b := agent.NewRemoteHumanFromExisting("B", agent.NewMemory(), agent.MaxEnergy)
	rt := New([]agent.Agent{a, b})

	a.RecvInput <- "ENTER_SIGNAL S000"
	b.RecvInput <- "ENTER_SIGNAL S001"
	rt.TickOnce()
	_ = mustRecvObs(t, a)
	_ = mustRecvObs(t, b)

	da := rt.dungeonByAgent[a.ID()]
	db := rt.dungeonByAgent[b.ID()]
	if da == nil || db == nil {
		t.Fatalf("expected both agents bound to dungeons")
	}
	if da == db {
		t.Fatalf("expected separate dungeon instances per agent")
	}
	da.ObjectiveType = "STABILIZE"
	da.ObjectiveTarget = 2
	da.ObjectiveProgress = 0
	db.ObjectiveType = "STABILIZE"
	db.ObjectiveTarget = 2
	db.ObjectiveProgress = 0

	rt.world.SetPosition(a.ID(), world.Position{X: da.Anchor.X, Y: da.Anchor.Y})
	rt.world.SetPosition(b.ID(), world.Position{X: db.Anchor.X + 1, Y: db.Anchor.Y})
	a.RecvInput <- "."
	b.RecvInput <- "."
	rt.TickOnce()
	_ = mustRecvObs(t, a)
	_ = mustRecvObs(t, b)

	if da.ObjectiveProgress != 1 {
		t.Fatalf("expected A progress 1, got %d", da.ObjectiveProgress)
	}
	if db.ObjectiveProgress != 0 {
		t.Fatalf("expected B progress 0, got %d", db.ObjectiveProgress)
	}
}
