package transport

import (
	"testing"
	"time"

	"github.com/divijg19/Nightshade/internal/agent"
)

func TestInMemoryTransportSnapshotDelivery(t *testing.T) {
	tx := NewInMemoryTransport(2)
	defer tx.Close()

	want := Snapshot{Observation: agent.Observation{Tick: 7}, Energy: 91}
	if err := tx.Publish(want); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case got := <-tx.Snapshots():
		if got.Observation.Tick != want.Observation.Tick || got.Energy != want.Energy {
			t.Fatalf("unexpected snapshot: %+v", got)
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("timed out waiting for snapshot")
	}
}

func TestInMemoryTransportCloseUnblocks(t *testing.T) {
	tx := NewInMemoryTransport(1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range tx.Snapshots() {
		}
	}()
	if err := tx.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatalf("snapshot reader did not exit")
	}
}
