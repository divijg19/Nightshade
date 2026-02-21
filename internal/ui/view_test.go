package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/divijg19/Nightshade/internal/agent"
)

func TestRouterDeterministicForSnapshot(t *testing.T) {
	in := make(chan tea.Msg, 1)
	m := NewModel(in, func(string) error { return nil })
	m.obs = agent.Observation{Mode: "board", Board: &agent.BoardView{Signals: []agent.SignalView{{ID: "S000", Type: "NULL"}}}}
	m.energy = 100
	m.hasObs = true
	v1 := m.View()
	v2 := m.View()
	if v1 != v2 {
		t.Fatalf("expected deterministic view output")
	}
}

func TestResizeNoPanic(t *testing.T) {
	in := make(chan tea.Msg, 1)
	m := NewModel(in, func(string) error { return nil })
	_, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
}

func TestSnapshotNotMutatedByView(t *testing.T) {
	in := make(chan tea.Msg, 1)
	m := NewModel(in, func(string) error { return nil })
	obs := agent.Observation{Mode: "board", Board: &agent.BoardView{Signals: []agent.SignalView{{ID: "S000", Type: "NULL", Corruption: 1}}}}
	m.obs = obs
	m.hasObs = true
	_ = m.View()
	if m.obs.Board == nil || len(m.obs.Board.Signals) != 1 || m.obs.Board.Signals[0].ID != "S000" {
		t.Fatalf("snapshot should not be mutated by view")
	}
}

func TestNoDuplicateEventLine(t *testing.T) {
	in := make(chan tea.Msg, 1)
	m := NewModel(in, func(string) error { return nil })
	m.obs = agent.Observation{Mode: "dungeon", Event: "Only once", Dungeon: &agent.DungeonView{Grid: [][]rune{[]rune("###"), []rune("#.#"), []rune("###")}, Pressure: 1, MaxPressure: 20, CoreIntegrity: 100, Threat: "LOW"}}
	m.energy = 100
	m.hasObs = true
	v := m.View()
	if strings.Count(v, "Only once") != 1 {
		t.Fatalf("expected exactly one event line")
	}
}
