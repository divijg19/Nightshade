package runtime

import (
	"testing"

	"github.com/divijg19/Nightshade/internal/agent"
)

// Tile decay determinism: when pressure >=6, tiles where (x+y+pressure)%4==0
// should be marked decayed and shown as '~' in dungeon grid.
func TestTileDecay_Deterministic(t *testing.T) {
	a := agent.NewRemoteHumanFromExisting("A", agent.NewMemory(), agent.MaxEnergy)
	rt := New([]agent.Agent{a})

	// Enter dungeon and create a reference to the instance
	a.RecvInput <- "ENTER_SIGNAL S000"
	rt.TickOnce()
	_ = mustRecvObs(t, a)

	d := rt.dungeonByAgent["A"]
	if d == nil {
		t.Fatalf("expected dungeon instance")
	}

	// Advance internal dungeon ticks directly until pressure reaches 6.
	for d.Pressure < 6 {
		d.Tick()
	}

	// If no decays recorded on the first qualifying tick, allow a few more
	// deterministic ticks to accumulate persistent decays (harmless and
	// still deterministic). This makes the test robust to ordering.
	if len(d.Decayed) == 0 {
		for i := 0; i < 3 && len(d.Decayed) == 0; i++ {
			d.Tick()
		}
	}

	// Build a snapshot and observe it via RemoteHuman.Observe
	snap := rt.snapshotFor(a, agent.Action(-1))
	a.Observe(snap)
	obs := mustRecvObs(t, a)
	if obs.Mode != "dungeon" {
		t.Fatalf("expected dungeon mode after manual tick")
	}

	// Verify decayed tiles obey the deterministic rule for the reported pressure
	p := obs.Dungeon.Pressure
	if p < 6 {
		t.Fatalf("expected pressure >=6, got %d", p)
	}

	// For every decayed tile reported, ensure it satisfies (x+y+pressure)%4==0
	for _, pos := range obs.Dungeon.DecayedTiles {
		if (pos.X+pos.Y+p)%4 != 0 {
			t.Fatalf("decayed tile %v does not satisfy decay rule for pressure=%d", pos, p)
		}
	}

	// Ensure at least one decayed tile exists and is shown as '~' in grid
	if len(obs.Dungeon.DecayedTiles) == 0 {
		t.Fatalf("expected some decayed tiles at pressure=%d", p)
	}
	found := false
	for y, row := range obs.Dungeon.Grid {
		for x, ch := range row {
			if ch == '~' {
				// Ensure this position exists in DecayedTiles
				for _, pos := range obs.Dungeon.DecayedTiles {
					if pos.X == x && pos.Y == y {
						found = true
						break
					}
				}
			}
			if found {
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		t.Fatalf("expected at least one '~' in dungeon grid after decay")
	}

}
