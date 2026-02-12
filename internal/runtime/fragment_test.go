package runtime

import "testing"

func TestCalculateFragments(t *testing.T) {
	// base=2, pressureBonus = maxPressure/3, bandBonus=instabilityBand, survivalBonus=2 if exited
	if v := CalculateFragments(0, 0, false); v != 2 {
		t.Fatalf("expected 2 got %d", v)
	}
	if v := CalculateFragments(9, 1, true); v != 2+(9/3)+1+2 {
		t.Fatalf("unexpected fragments: %d", v)
	}
}
