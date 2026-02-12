package runtime

import (
	"testing"

	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/persist"
)

func TestMultiplayerIsolationProgress(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NIGHTSHADE_DIR", dir)

	a := "M-A"
	b := "M-B"
	pa := persist.DefaultProgress()
	pa.Fragments = 10
	pa.UnlockedSkills["endurance_1"] = true
	pb := persist.DefaultProgress()
	pb.Fragments = 30
	pb.UnlockedSkills["pressure_sense"] = true
	if err := persist.SaveProgress(a, pa); err != nil {
		t.Fatalf("save a: %v", err)
	}
	if err := persist.SaveProgress(b, pb); err != nil {
		t.Fatalf("save b: %v", err)
	}

	aa := agent.NewRemoteHumanFromExisting(a, agent.NewMemory(), agent.MaxEnergy)
	bb := agent.NewRemoteHumanFromExisting(b, agent.NewMemory(), agent.MaxEnergy)
	rt := New([]agent.Agent{aa, bb})

	// ensure runtime loaded distinct progress
	paLoaded := rt.progressByAgent[a]
	pbLoaded := rt.progressByAgent[b]
	if paLoaded == nil || pbLoaded == nil {
		t.Fatalf("missing progress in runtime")
	}
	if paLoaded.UnlockedSkills["endurance_1"] == pbLoaded.UnlockedSkills["endurance_1"] {
		// endurance_1 true only for A
		if paLoaded.UnlockedSkills["endurance_1"] == pbLoaded.UnlockedSkills["endurance_1"] {
			// fine check next
		}
	}
	if !paLoaded.UnlockedSkills["endurance_1"] {
		t.Fatalf("expected A to have endurance_1 unlocked")
	}
	if !pbLoaded.UnlockedSkills["pressure_sense"] {
		t.Fatalf("expected B to have pressure_sense unlocked")
	}
}
