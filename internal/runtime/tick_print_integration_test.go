package runtime

import (
	"testing"

	"github.com/divijg19/Nightshade/internal/agent"
)

func TestTickOnce_PrintsPerAgentSnapshot(t *testing.T) {
	a1 := agent.NewScripted("A")
	a2 := agent.NewOscillating("B")

	rt := New([]agent.Agent{a1, a2})
	// Use SnapshotForDebug to verify snapshots (keeps debug functions in tests).
	snapA, ok := rt.SnapshotForDebug("A")
	if !ok {
		t.Fatal("missing snapshot for A")
	}
	if snapA.Tick != 0 || snapA.Position.X != 0 || snapA.Position.Y != 0 || snapA.Health != 0 || snapA.Energy != 0 {
		t.Fatalf("unexpected snapshot for A: %+v", snapA)
	}

	snapB, ok := rt.SnapshotForDebug("B")
	if !ok {
		t.Fatal("missing snapshot for B")
	}
	if snapB.Tick != 0 || snapB.Position.X != 1 || snapB.Position.Y != 0 || snapB.Health != 0 || snapB.Energy != 0 {
		t.Fatalf("unexpected snapshot for B: %+v", snapB)
	}
}
