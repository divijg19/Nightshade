package runtime

import "testing"

func TestCalculateFragments(t *testing.T) {
	if v := CalculateFragments(0, 0, false, false, 100); v != 4 {
		t.Fatalf("expected 4 got %d", v)
	}
	if v := CalculateFragments(9, 1, true, true, 50); v != 2+3+(9/3)+1 {
		t.Fatalf("unexpected fragments: %d", v)
	}
}
