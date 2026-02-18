package runtime

import (
	"strings"
	"testing"

	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/persist"
)

func TestOnboardingOverlayShownOnFirstDungeonOnly(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NIGHTSHADE_DIR", dir)

	a := agent.NewRemoteHumanFromExisting("A", agent.NewMemory(), agent.MaxEnergy)
	rt := New([]agent.Agent{a})
	a.RecvInput <- "ENTER_SIGNAL S000"
	rt.TickOnce()
	_ = mustRecvObs(t, a)
	a.RecvInput <- "."
	rt.TickOnce()
	obs := mustRecvObs(t, a)
	if !strings.Contains(obs.Event, "You are entering a signal fragment.") {
		t.Fatalf("expected onboarding overlay on first entry, got %q", obs.Event)
	}

	p, err := persist.LoadProgress("A")
	if err != nil {
		t.Fatalf("load progress: %v", err)
	}
	if !p.DungeonIntroShown {
		t.Fatalf("expected onboarding flag persisted")
	}

	a2 := agent.NewRemoteHumanFromExisting("A", agent.NewMemory(), agent.MaxEnergy)
	rt2 := New([]agent.Agent{a2})
	a2.RecvInput <- "ENTER_SIGNAL S001"
	rt2.TickOnce()
	_ = mustRecvObs(t, a2)
	a2.RecvInput <- "."
	rt2.TickOnce()
	obs2 := mustRecvObs(t, a2)
	if strings.Contains(obs2.Event, "You are entering a signal fragment.") {
		t.Fatalf("expected onboarding overlay not to repeat")
	}
}
