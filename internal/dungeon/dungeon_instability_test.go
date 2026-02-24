package dungeon

import "testing"

func TestInstabilityBand_Boundaries(t *testing.T) {
	d := NewInstance("D", AnchorMemoryVault)

	table := []struct {
		pressure int
		want     int
	}{
		{pressure: 0, want: 0},
		{pressure: 5, want: 0},
		{pressure: 6, want: 1},
		{pressure: 10, want: 1},
		{pressure: 11, want: 2},
		{pressure: 15, want: 2},
		{pressure: 16, want: 3},
		{pressure: 999, want: 3},
	}

	for _, tc := range table {
		d.Pressure = tc.pressure
		if got := d.InstabilityBand(); got != tc.want {
			t.Fatalf("pressure=%d band=%d want=%d", tc.pressure, got, tc.want)
		}
	}
}

func TestInstabilityBand_MonotonicWithPressure(t *testing.T) {
	d := NewInstance("D", AnchorMemoryVault)
	prev := d.InstabilityBand()
	for i := 0; i < 30; i++ {
		d.Tick()
		got := d.InstabilityBand()
		if got < prev {
			t.Fatalf("band decreased at pressure=%d: got=%d prev=%d", d.Pressure, got, prev)
		}
		prev = got
	}
}

func TestInstabilityBand_ResetsOnNewDungeon(t *testing.T) {
	d1 := NewInstance("D1", AnchorMemoryVault)
	for i := 0; i < 20; i++ {
		d1.Tick()
	}
	if d1.InstabilityBand() != 3 {
		t.Fatalf("expected d1 to reach critical band")
	}

	d2 := NewInstance("D2", AnchorMemoryVault)
	if d2.Pressure != 0 {
		t.Fatalf("expected new dungeon pressure=0")
	}
	if d2.InstabilityBand() != 0 {
		t.Fatalf("expected new dungeon band=0")
	}
}

func TestDefaultDungeonDimensionsAre20x20(t *testing.T) {
	d := NewInstance("D", AnchorMemoryVault)
	if d.Width != 20 || d.Height != 20 {
		t.Fatalf("expected 20x20 dungeon, got %dx%d", d.Width, d.Height)
	}
	if d.Entry.X != 10 || d.Entry.Y != 19 {
		t.Fatalf("unexpected entry position: %+v", d.Entry)
	}
	if d.Anchor.X != 10 || d.Anchor.Y != 10 {
		t.Fatalf("unexpected anchor position: %+v", d.Anchor)
	}
	if d.Exit.X != 10 || d.Exit.Y != 0 {
		t.Fatalf("unexpected exit position: %+v", d.Exit)
	}
}
