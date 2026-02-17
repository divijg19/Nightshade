package runtime

import (
	"testing"

	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/core"
	"github.com/divijg19/Nightshade/internal/dungeon"
	"github.com/divijg19/Nightshade/internal/world"
)

func setupV0310RT(t *testing.T) (*Runtime, *agent.RemoteHuman) {
	t.Helper()
	a := agent.NewRemoteHumanFromExisting("A", agent.NewMemory(), agent.MaxEnergy)
	rt := New([]agent.Agent{a})
	a.RecvInput <- "ENTER_SIGNAL S000"
	rt.TickOnce()
	_ = mustRecvObs(t, a)
	return rt, a
}

func TestV0310_RiskNodeImpactNarration(t *testing.T) {
	rt, a := setupV0310RT(t)
	d := rt.dungeonByAgent[a.ID()]
	d.FragmentNode = core.Position{X: 3, Y: 4}
	rt.world.SetPosition(a.ID(), world.Position{X: 3, Y: 5})
	a.RecvInput <- "w"
	rt.TickOnce()
	snap := rt.snapshotFor(a, agent.Action(-1))
	if snap.Event != "Instability surges (+2)." {
		t.Fatalf("expected risk node narration, got %q", snap.Event)
	}
}

func TestV0310_ExitFailureNarration(t *testing.T) {
	rt, a := setupV0310RT(t)
	d := rt.dungeonByAgent[a.ID()]
	d.ObjectiveCompleted = false
	rt.world.SetPosition(a.ID(), world.Position{X: d.Exit.X, Y: d.Exit.Y + 1})
	a.RecvInput <- "w"
	rt.TickOnce()
	snap := rt.snapshotFor(a, agent.Action(-1))
	if snap.Event != "Exit blocked — objective incomplete." {
		t.Fatalf("expected exit blocked narration, got %q", snap.Event)
	}
}

func TestV0310_ChannelBreakNarration(t *testing.T) {
	rt, a := setupV0310RT(t)
	d := rt.dungeonByAgent[a.ID()]
	d.Pressure = 16
	rt.world.SetPosition(a.ID(), world.Position{X: d.Exit.X, Y: d.Exit.Y + 1})
	a.RecvInput <- "w"
	rt.TickOnce()
	if len(d.Entities) == 0 {
		d.Entities = append(d.Entities, dungeon.Entity{ID: "e-0", Pos: d.Anchor, Kind: dungeon.EnemyHunter})
	}
	d.Entities[0].TargetLocked = true
	a.RecvInput <- "."
	rt.TickOnce()
	snap := rt.snapshotFor(a, agent.Action(-1))
	if snap.Event != "Channel broken by threat." {
		t.Fatalf("expected channel-break narration, got %q", snap.Event)
	}
}

func TestV0310_EnergyClampMovementWaitOvercharge(t *testing.T) {
	rt, a := setupV0310RT(t)
	d := rt.dungeonByAgent[a.ID()]
	d.Entities = nil
	a.AdjustEnergy(-10)
	start := a.Energy()

	// move costs exactly 1
	a.RecvInput <- "w"
	rt.TickOnce()
	_ = mustRecvObs(t, a)
	afterMove := a.Energy()
	if afterMove != start-1 {
		t.Fatalf("expected move cost 1, got delta %d", afterMove-start)
	}

	// wait restores exactly 1 when not locked/critical
	a.RecvInput <- "."
	rt.TickOnce()
	_ = mustRecvObs(t, a)
	afterWait := a.Energy()
	if afterWait != afterMove+1 {
		t.Fatalf("expected wait restore 1, got delta %d", afterWait-afterMove)
	}

	// overcharge grants exactly +2 (in addition to this tick's move cost)
	d.OverchargeNode = core.Position{X: 1, Y: 1}
	rt.world.SetPosition(a.ID(), world.Position{X: 1, Y: 2})
	beforeOvercharge := a.Energy()
	a.RecvInput <- "w"
	rt.TickOnce()
	_ = mustRecvObs(t, a)
	afterOvercharge := a.Energy()
	if afterOvercharge != beforeOvercharge+1 {
		t.Fatalf("expected net +1 with move(-1) + overcharge(+2), got delta %d", afterOvercharge-beforeOvercharge)
	}
}
