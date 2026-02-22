package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/divijg19/Nightshade/internal/agent"
)

type stubNet struct {
	sent       []string
	disconnect int
}

func (s *stubNet) SendInput(key string) error {
	s.sent = append(s.sent, key)
	return nil
}

func (s *stubNet) Disconnect() error {
	s.disconnect++
	return nil
}

func TestRouterDeterministicForSnapshot(t *testing.T) {
	in := make(chan tea.Msg, 1)
	net := &stubNet{}
	m := NewModel(in, net)
	m.obs = agent.Observation{Mode: "board", Board: &agent.BoardView{Signals: []agent.SignalView{{ID: "S000", Type: "NULL"}}}}
	m.energy = 100
	m.hasObs = true
	v1 := m.View()
	v2 := m.View()
	if v1 != v2 {
		t.Fatalf("expected deterministic view output")
	}
}

func TestResizeUpdatesModelState(t *testing.T) {
	in := make(chan tea.Msg, 1)
	m := NewModel(in, &stubNet{})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	next := updated.(Model)
	if next.width != 120 || next.height != 40 {
		t.Fatalf("expected width/height to update, got %d x %d", next.width, next.height)
	}
}

func TestSnapshotMsgUpdatesModelState(t *testing.T) {
	in := make(chan tea.Msg, 1)
	m := NewModel(in, &stubNet{})
	obs := agent.Observation{Mode: "board", Tick: 9, Board: &agent.BoardView{Signals: []agent.SignalView{{ID: "S000", Type: "NULL"}}}}
	updated, _ := m.Update(SnapshotMsg{Obs: obs, Energy: 73})
	next := updated.(Model)
	if !next.hasObs {
		t.Fatalf("expected model to have snapshot")
	}
	if next.energy != 73 {
		t.Fatalf("expected energy 73, got %d", next.energy)
	}
	if next.obs.Tick != 9 {
		t.Fatalf("expected tick 9, got %d", next.obs.Tick)
	}
}

func TestSnapshotNotMutatedByView(t *testing.T) {
	in := make(chan tea.Msg, 1)
	m := NewModel(in, &stubNet{})
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
	m := NewModel(in, &stubNet{})
	m.obs = agent.Observation{Mode: "dungeon", Event: "Only once", Dungeon: &agent.DungeonView{Grid: [][]rune{[]rune("###"), []rune("#.#"), []rune("###")}, Pressure: 1, MaxPressure: 20, CoreIntegrity: 100, Threat: "LOW"}}
	m.energy = 100
	m.hasObs = true
	v := m.View()
	if strings.Count(v, "Only once") != 1 {
		t.Fatalf("expected exactly one event line")
	}
}

func TestPressureBarWidthStable(t *testing.T) {
	b1 := pressureBar(1, 20, 12)
	b2 := pressureBar(19, 20, 12)
	if len([]rune(b1)) != len([]rune(b2)) {
		t.Fatalf("expected stable bar width, got %q vs %q", b1, b2)
	}
	if len([]rune(b1)) != 14 {
		t.Fatalf("expected bar width 14 including brackets, got %d", len([]rune(b1)))
	}
}

func TestMinimumSizeFallback(t *testing.T) {
	in := make(chan tea.Msg, 1)
	m := NewModel(in, &stubNet{})
	m.width = 20
	m.height = 5
	m.hasObs = true
	m.obs = agent.Observation{Mode: "dungeon", Dungeon: &agent.DungeonView{Grid: [][]rune{[]rune("###")}}}
	v := m.View()
	if !strings.Contains(v, "TERMINAL TOO SMALL") {
		t.Fatalf("expected fallback message for small terminal")
	}
}

func TestHeaderConsistencyContainsLabels(t *testing.T) {
	in := make(chan tea.Msg, 1)
	m := NewModel(in, &stubNet{})
	m.width = 120
	m.height = 30
	m.energy = 88
	m.hasObs = true
	m.obs = agent.Observation{Mode: "dungeon", Dungeon: &agent.DungeonView{Grid: [][]rune{[]rune("###")}, InstabilityLabel: "CRITICAL", Pressure: 5, MaxPressure: 20, CoreIntegrity: 97, Threat: "HIGH"}}
	v := m.View()
	if !strings.Contains(v, "MODE:") || !strings.Contains(v, "SIGNAL:") || !strings.Contains(v, "INSTABILITY:") || !strings.Contains(v, "ENERGY:") {
		t.Fatalf("expected header labels to be present")
	}
}
