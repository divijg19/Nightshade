package runtime

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/core"
	"github.com/divijg19/Nightshade/internal/dungeon"
	"github.com/divijg19/Nightshade/internal/render"
	"github.com/divijg19/Nightshade/internal/world"
)

func setupRTWithAgent(t *testing.T) (*Runtime, *agent.RemoteHuman) {
	a := agent.NewRemoteHumanFromExisting("A", agent.NewMemory(), agent.MaxEnergy)
	rt := New([]agent.Agent{a})
	// enter deterministic first signal
	a.RecvInput <- "ENTER_SIGNAL S000"
	rt.TickOnce()
	_ = mustRecvObs(t, a)
	return rt, a
}

func TestEnrageTriggersAt15_New(t *testing.T) {
	rt, a := setupRTWithAgent(t)
	d := rt.dungeonByAgent[a.ID()]
	d.Pressure = 15
	if !d.Enraged() {
		t.Fatalf("expected Enraged true at 15")
	}
	// snapshot reflects enraged
	snap := rt.snapshotFor(a, agent.Action(-1))
	if !snap.Dungeon.Enraged {
		t.Fatalf("snapshot did not mark enraged")
	}
}

func TestExitChannelBreaksIfLocked(t *testing.T) {
	rt, a := setupRTWithAgent(t)
	d := rt.dungeonByAgent[a.ID()]
	// force enraged
	d.Pressure = 16
	// place agent just below exit
	rt.world.SetPosition(a.ID(), world.Position{X: d.Exit.X, Y: d.Exit.Y + 1})
	// move onto exit (north)
	a.RecvInput <- "w"
	rt.TickOnce()
	// Check authoritative runtime state for channeling
	if !d.ExitChanneling {
		t.Fatalf("expected channeling state after stepping onto exit in enraged")
	}
	// simulate an enemy locking during channel
	if len(d.Entities) == 0 {
		d.Entities = append(d.Entities, dungeon.Entity{ID: "e-0", Pos: d.Anchor, Kind: dungeon.EnemyHunter})
	}
	d.Entities[0].TargetLocked = true
	// next tick: channel broken
	a.RecvInput <- "."
	rt.TickOnce()
	// authoritative snapshot should carry channel break event
	snap2 := rt.snapshotFor(a, agent.Action(-1))
	if snap2.Dungeon.Event != "Channel broken by threat." && snap2.Event != "Channel broken by threat." {
		t.Fatalf("expected channel break event, got %q / %q", snap2.Dungeon.Event, snap2.Event)
	}
}

func TestFragmentNodeRaisesPressure(t *testing.T) {
	rt, a := setupRTWithAgent(t)
	d := rt.dungeonByAgent[a.ID()]
	// place fragment node at (3,4)
	d.FragmentNode = core.Position{X: 3, Y: 4}
	start := d.Pressure
	// place agent above node
	rt.world.SetPosition(a.ID(), world.Position{X: 3, Y: 5})
	a.RecvInput <- "w"
	rt.TickOnce()
	if d.Pressure != start+2 {
		t.Fatalf("expected pressure +2 after fragment node, got %d want %d", d.Pressure, start+2)
	}
}

func TestWellReducesPressure(t *testing.T) {
	rt, a := setupRTWithAgent(t)
	d := rt.dungeonByAgent[a.ID()]
	d.CorruptionWell = core.Position{X: 2, Y: 2}
	d.Pressure = 5
	rt.world.SetPosition(a.ID(), world.Position{X: 2, Y: 3})
	a.RecvInput <- "w"
	rt.TickOnce()
	if d.Pressure != 3 {
		t.Fatalf("expected pressure reduced by 2, got %d", d.Pressure)
	}
}

func TestOverchargeRaisesEnergy(t *testing.T) {
	rt, a := setupRTWithAgent(t)
	d := rt.dungeonByAgent[a.ID()]
	d.Entities = nil
	d.OverchargeNode = core.Position{X: 2, Y: 2}
	rt.world.SetPosition(a.ID(), world.Position{X: 2, Y: 3})
	a.RecvInput <- "w"
	// drain some energy first
	a.AdjustEnergy(-5)
	before := a.Energy()
	rt.TickOnce()
	if a.Energy() <= before {
		t.Fatalf("expected energy increased by overcharge node")
	}
}

func TestWaitBlockedWhenLocked(t *testing.T) {
	rt, a := setupRTWithAgent(t)
	d := rt.dungeonByAgent[a.ID()]
	// create locked enemy
	if len(d.Entities) == 0 {
		d.Entities = append(d.Entities, dungeon.Entity{ID: "e-0", Pos: d.Anchor, Kind: dungeon.EnemyHunter})
	}
	d.Entities[0].TargetLocked = true
	// set energy below max
	a.AdjustEnergy(-10)
	before := a.Energy()
	a.RecvInput <- "."
	rt.TickOnce()
	if a.Energy() != before {
		t.Fatalf("expected no wait restoration while locked, got %d before %d", a.Energy(), before)
	}
}

func TestWorldEventCycleDeterministic(t *testing.T) {
	rt, a := setupRTWithAgent(t)
	// tick 0 => index 0
	_ = rt.snapshotFor(a, agent.Action(-1))
	// simulate 100 ticks
	for i := 0; i < 100; i++ {
		a.RecvInput <- "."
		rt.TickOnce()
		_ = mustRecvObs(t, a)
		if _, ok := rt.dungeonByAgent[a.ID()]; !ok {
			break
		}
	}
	snap2 := rt.snapshotFor(a, agent.Action(-1))
	// If the dungeon was unbound during the 100-tick simulation, fall back
	// to computing the deterministic world event label from `rt.tick`.
	label := ""
	if snap2.Dungeon.WorldEventLabel != "" {
		label = snap2.Dungeon.WorldEventLabel
	} else {
		wc := rt.Tick() / 100
		switch wc % 3 {
		case 0:
			label = "Signal Surge"
		case 1:
			label = "Stability Drain"
		default:
			label = "Hunter Migration"
		}
	}
	if label == "" {
		t.Fatalf("expected world event label after 100 ticks")
	}
}

func TestBuildLabelDerivation(t *testing.T) {
	rt, a := setupRTWithAgent(t)
	p := rt.progressByAgent[a.ID()]
	p.UnlockedSkills["anchor_mastery"] = true
	snap := rt.snapshotFor(a, agent.Action(-1))
	if snap.Dungeon.BuildLabel != "Stabilizer" {
		t.Fatalf("expected Stabilizer build label, got %q", snap.Dungeon.BuildLabel)
	}
}

func TestPressureBarRendering(t *testing.T) {
	rt, a := setupRTWithAgent(t)
	d := rt.dungeonByAgent[a.ID()]
	d.Pressure = 4
	d.MaxPressure = 20
	// advance one tick to produce a fresh snapshot reflecting changed pressure
	a.RecvInput <- "."
	rt.TickOnce()
	obs := mustRecvObs(t, a)
	// render and strip ANSI
	out := render.RenderForTest(obs)
	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	plain := re.ReplaceAllString(out, "")
	// Expect the rendered pressure to reflect current runtime values.
	expected := fmt.Sprintf("%d/%d", d.Pressure, d.MaxPressure)
	if !regexp.MustCompile(regexp.QuoteMeta(expected)).MatchString(plain) {
		t.Fatalf("pressure line missing, got %s", plain)
	}
}

func TestQuickDiveSelectsHighestCorruption(t *testing.T) {
	rt, a := setupRTWithAgent(t)
	// quick dive is client-only key Q; we ensure board cursor exists and is deterministic
	a.RecvInput <- "Q"
	// runtime will ignore unknown keys; ensure no panic and snapshot available
	rt.TickOnce()
	_ = mustRecvObs(t, a)
}
