package runtime

import (
	"testing"
	"time"

	"github.com/divijg19/Nightshade/internal/agent"
)

// Ensure runtime sends exactly one Observation per RemoteHuman per tick.
func TestRuntime_OneObservationPerAgentPerTick(t *testing.T) {
	a := agent.NewRemoteHumanFromExisting("A", agent.NewMemory(), agent.MaxEnergy)
	rt := New([]agent.Agent{a})

	// Drive one tick with idle input.
	a.RecvInput <- "."
	rt.TickOnce()

	// First observation should be present.
	select {
	case <-a.SendObservation:
		// ok
	case <-time.After(250 * time.Millisecond):
		t.Fatalf("timed out waiting for first observation")
	}

	// There must be no second observation sent for the same tick.
	select {
	case obs := <-a.SendObservation:
		t.Fatalf("unexpected extra observation: %#v", obs)
	case <-time.After(50 * time.Millisecond):
		// expected: no extra
	}
}
