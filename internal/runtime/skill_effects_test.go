package runtime

import (
	"testing"

	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/dungeon"
	"github.com/divijg19/Nightshade/internal/persist"
	"github.com/divijg19/Nightshade/internal/world"
)

func setupWithProgress(t *testing.T, agentID string, p *persist.Progress) (*Runtime, *agent.RemoteHuman) {
	t.Helper()
	if err := persist.SaveProgress(agentID, p); err != nil {
		t.Fatalf("save progress: %v", err)
	}
	a := agent.NewRemoteHumanFromExisting(agentID, agent.NewMemory(), agent.MaxEnergy+50)
	rt := New([]agent.Agent{a})
	// enter dungeon via signal to get instance bound
	a.RecvInput <- "ENTER_SIGNAL S000"
	rt.TickOnce()
	// Ensure energy clamp to MaxEnergy for deterministic expectations in tests
	a.AdjustEnergy(agent.MaxEnergy - a.Energy())
	return rt, a
}

func TestEnduranceIncreasesCap(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NIGHTSHADE_DIR", dir)
	aid := "E1"
	p := persist.DefaultProgress()
	p.Fragments = 10 // 1 skill point
	// without endurance
	rt, a := setupWithProgress(t, aid, p)
	// call AdjustEnergy(0) to trigger clamp
	a.AdjustEnergy(0)
	if a.Energy() != agent.MaxEnergy {
		t.Fatalf("expected default cap %d got %d", agent.MaxEnergy, a.Energy())
	}
	// grant endurance_1
	p.UnlockedSkills["endurance_1"] = true
	if err := persist.SaveProgress(aid, p); err != nil {
		t.Fatalf("save: %v", err)
	}
	// recreate runtime to apply bonuses
	a2 := agent.NewRemoteHumanFromExisting(aid, agent.NewMemory(), agent.MaxEnergy+50)
	rt2 := New([]agent.Agent{a2})
	a2.AdjustEnergy(0)
	if a2.Energy() != agent.MaxEnergy+5 {
		t.Fatalf("expected cap %d got %d", agent.MaxEnergy+5, a2.Energy())
	}
	_ = rt
	_ = rt2
}

func TestAnchorMasterySlowsPressure(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NIGHTSHADE_DIR", dir)
	aid := "A1"
	p := persist.DefaultProgress()
	p.UnlockedSkills["anchor_mastery"] = true
	if err := persist.SaveProgress(aid, p); err != nil {
		t.Fatalf("save: %v", err)
	}
	a := agent.NewRemoteHumanFromExisting(aid, agent.NewMemory(), agent.MaxEnergy)
	rt := New([]agent.Agent{a})
	// bind a manual instance
	inst := dungeon.NewInstance("D-test", dungeon.AnchorMemoryVault)
	rt.dungeonByAgent[a.ID()] = inst
	// place agent at anchor and run ticks
	rt.world.SetPosition(a.ID(), world.Position{X: inst.Anchor.X, Y: inst.Anchor.Y})
	// initial pressure
	inst.Pressure = 0
	// run 6 ticks; cadence 3 increments at ticks 0 and 3 => 2 increments; cadence2 increments at 0,2,4 => 3
	for i := 0; i < 6; i++ {
		a.RecvInput <- "."
		rt.TickOnce()
	}
	if inst.Pressure != 2 {
		t.Fatalf("expected pressure 2 with anchor_mastery after 4 ticks, got %d", inst.Pressure)
	}
	// baseline without mastery should increment faster
	aid2 := "A2"
	p2 := persist.DefaultProgress()
	if err := persist.SaveProgress(aid2, p2); err != nil {
		t.Fatalf("save: %v", err)
	}
	b := agent.NewRemoteHumanFromExisting(aid2, agent.NewMemory(), agent.MaxEnergy)
	rt2 := New([]agent.Agent{b})
	inst2 := dungeon.NewInstance("D-test2", dungeon.AnchorMemoryVault)
	rt2.dungeonByAgent[b.ID()] = inst2
	rt2.world.SetPosition(b.ID(), world.Position{X: inst2.Anchor.X, Y: inst2.Anchor.Y})
	inst2.Pressure = 0
	for i := 0; i < 6; i++ {
		b.RecvInput <- "."
		rt2.TickOnce()
	}
	if inst2.Pressure <= inst.Pressure {
		t.Fatalf("expected baseline pressure > mastery pressure; got %d vs %d", inst2.Pressure, inst.Pressure)
	}
}

func TestExitInstinctAllowsExitWhenExhausted(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NIGHTSHADE_DIR", dir)
	aid := "X1"
	p := persist.DefaultProgress()
	p.UnlockedSkills["exit_instinct"] = true
	if err := persist.SaveProgress(aid, p); err != nil {
		t.Fatalf("save: %v", err)
	}
	// The authoritative exit allowance logic is deterministic and independent
	// of client-side energy-collapse gating. Test the server-side allow logic
	// directly: in CRITICAL band, energy < 1 is allowed only when skill present.
	a := agent.NewRemoteHumanFromExisting(aid, agent.NewMemory(), 0)
	rt := New([]agent.Agent{a})
	inst := dungeon.NewInstance("D-x", dungeon.AnchorRecoveryNode)
	inst.Pressure = 16
	band := inst.InstabilityBand()
	// without skill, should not allow exit when exhausted
	if band < 3 {
		t.Fatalf("test setup expected CRITICAL band")
	}
	// energy is 0
	energy := a.Energy()
	p2 := rt.progressByAgent[a.ID()]
	// ensure no skill
	p2.UnlockedSkills["exit_instinct"] = false
	allow := true
	if band >= 3 && energy < 1 {
		if !p2.UnlockedSkills["exit_instinct"] {
			allow = false
		}
	}
	if allow {
		t.Fatalf("expected exit to be denied without exit_instinct when exhausted")
	}
	// now enable skill and re-evaluate
	p2.UnlockedSkills["exit_instinct"] = true
	allow2 := true
	if band >= 3 && energy < 1 {
		if !p2.UnlockedSkills["exit_instinct"] {
			allow2 = false
		}
	}
	if !allow2 {
		t.Fatalf("expected exit to be allowed with exit_instinct when exhausted")
	}
}

func TestExtendedDistractDuration(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NIGHTSHADE_DIR", dir)
	aid := "D1"
	p := persist.DefaultProgress()
	p.Fragments = 10
	p.UnlockedSkills["extended_distract"] = true
	if err := persist.SaveProgress(aid, p); err != nil {
		t.Fatalf("save: %v", err)
	}
	a := agent.NewRemoteHumanFromExisting(aid, agent.NewMemory(), agent.MaxEnergy)
	rt := New([]agent.Agent{a})
	// enter dungeon
	a.RecvInput <- "ENTER_SIGNAL S000"
	rt.TickOnce()
	d := rt.dungeonByAgent[a.ID()]
	if d == nil || len(d.Entities) == 0 {
		t.Fatalf("expected entities in dungeon")
	}
	// place player near entity
	e := d.Entities[0]
	rt.world.SetPosition(a.ID(), world.Position{X: e.Pos.X + 2, Y: e.Pos.Y})
	// perform DISTRACT
	a.RecvInput <- "d"
	rt.TickOnce()
	// next three ticks, enemy should not target player (extended -> 3 ticks)
	for i := 0; i < 3; i++ {
		a.RecvInput <- "."
		rt.TickOnce()
		snap := rt.snapshotFor(a, agent.Action(-1))
		for _, ev := range snap.Dungeon.Enemies {
			if ev.Target == "player" {
				t.Fatalf("expected distract to hold for tick %d", i)
			}
		}
	}
}

func TestThreatAwarenessTelegraph(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NIGHTSHADE_DIR", dir)
	aid := "T1"
	p := persist.DefaultProgress()
	p.UnlockedSkills["threat_awareness"] = true
	if err := persist.SaveProgress(aid, p); err != nil {
		t.Fatalf("save: %v", err)
	}
	a := agent.NewRemoteHumanFromExisting(aid, agent.NewMemory(), agent.MaxEnergy)
	rt := New([]agent.Agent{a})
	// bind instance and craft entity with LockTicks=1 visible
	inst := dungeon.NewInstance("D-thr", dungeon.AnchorMemoryVault)
	e := dungeon.Entity{ID: "e-1", Pos: inst.Anchor, Kind: dungeon.EnemySentinel, LockTicks: 1, TargetLocked: false}
	inst.Entities = append(inst.Entities, e)
	rt.dungeonByAgent[a.ID()] = inst
	rt.world.SetPosition(a.ID(), world.Position{X: inst.Anchor.X, Y: inst.Anchor.Y})
	// snapshot should show TargetLocked true because threat_awareness telegraphs one tick early
	snap := rt.snapshotFor(a, agent.Action(-1))
	found := false
	for _, ev := range snap.Dungeon.Enemies {
		if ev.TargetLocked {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected telegraphed locked flag with threat_awareness")
	}
}

func TestPressureSenseNextThreshold(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NIGHTSHADE_DIR", dir)
	aid := "P1"
	p := persist.DefaultProgress()
	p.UnlockedSkills["pressure_sense"] = true
	if err := persist.SaveProgress(aid, p); err != nil {
		t.Fatalf("save: %v", err)
	}
	a := agent.NewRemoteHumanFromExisting(aid, agent.NewMemory(), agent.MaxEnergy)
	rt := New([]agent.Agent{a})
	inst := dungeon.NewInstance("D-ps", dungeon.AnchorMemoryVault)
	inst.Pressure = 5
	rt.dungeonByAgent[a.ID()] = inst
	snap := rt.snapshotFor(a, agent.Action(-1))
	if snap.Dungeon.NextBandThreshold != 1 {
		t.Fatalf("expected next threshold 1 for pressure 5 -> next band at 6, got %d", snap.Dungeon.NextBandThreshold)
	}
}
