package runtime

import (
	"encoding/json"
	"testing"
)

func TestSnapshotBackwardCompatibilityUnmarshal(t *testing.T) {
	oldJSON := `{"Tick":1,"SelfID":"A","Position":{"X":0,"Y":0},"Mode":"board","Board":{"cursor":0,"signals":[]}}`
	var snap Snapshot
	if err := json.Unmarshal([]byte(oldJSON), &snap); err != nil {
		t.Fatalf("expected old snapshot JSON to unmarshal: %v", err)
	}
	if snap.Mode != "board" {
		t.Fatalf("expected board mode after unmarshal")
	}
}
