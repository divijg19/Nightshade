package agent

import "testing"

func TestOscillatingAgent_Parity(t *testing.T) {
    o := NewOscillating("O")

    // Build a minimal snapshot-like value containing Tick. Decide uses
    // reflection to read the Tick field, so any value with exported Tick int
    // will work here without importing runtime (avoids cycles).
    evenSnap := struct{ Tick int }{Tick: 2}
    act := o.Decide(evenSnap)
    if act != MOVE_N {
        t.Fatalf("even tick: got %v, want MOVE_N", act)
    }

    oddSnap := struct{ Tick int }{Tick: 3}
    act = o.Decide(oddSnap)
    if act != MOVE_S {
        t.Fatalf("odd tick: got %v, want MOVE_S", act)
    }
}
