package agent

import "testing"

type testSnap struct{ t int }

func (s testSnap) TickValue() int { return s.t }

func TestOscillatingAgent_Parity(t *testing.T) {
	o := NewOscillating("O")

	evenSnap := testSnap{t: 2}
	act := o.Decide(evenSnap)
	if act != MOVE_N {
		t.Fatalf("even tick: got %v, want MOVE_N", act)
	}

	oddSnap := testSnap{t: 3}
	act = o.Decide(oddSnap)
	if act != MOVE_S {
		t.Fatalf("odd tick: got %v, want MOVE_S", act)
	}
}
