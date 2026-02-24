package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/core"
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
	m := NewModel(in, net, ModelOptions{})
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
	m := NewModel(in, &stubNet{}, ModelOptions{})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	next := updated.(Model)
	if next.width != 120 || next.height != 40 {
		t.Fatalf("expected width/height to update, got %d x %d", next.width, next.height)
	}
}

func TestSnapshotMsgUpdatesModelState(t *testing.T) {
	in := make(chan tea.Msg, 1)
	m := NewModel(in, &stubNet{}, ModelOptions{})
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
	m := NewModel(in, &stubNet{}, ModelOptions{})
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
	m := NewModel(in, &stubNet{}, ModelOptions{})
	m.obs = agent.Observation{Mode: "dungeon", Event: "Only once", Dungeon: &agent.DungeonView{Grid: [][]rune{[]rune("###"), []rune("#.#"), []rune("###")}, Pressure: 1, MaxPressure: 20, CoreIntegrity: 100, Threat: "LOW"}}
	m.energy = 100
	m.hasObs = true
	v := m.View()
	if strings.Count(v, "Only once") != 1 {
		t.Fatalf("expected exactly one event line")
	}
}

func TestPressureBarWidthStable(t *testing.T) {
	SetPresentationOptions(PresentationOptions{ASCIIMode: false, ColorLevel: ColorTrue})
	b1 := pressureBar(1, 20, 12)
	b2 := pressureBar(19, 20, 12)
	if displayWidth(b1) != displayWidth(b2) {
		t.Fatalf("expected stable bar width, got %q vs %q", b1, b2)
	}
	if displayWidth(b1) != 14 {
		t.Fatalf("expected bar width 14 including brackets, got %d", displayWidth(b1))
	}
}

func TestMinimumSizeFallback(t *testing.T) {
	in := make(chan tea.Msg, 1)
	m := NewModel(in, &stubNet{}, ModelOptions{})
	m.width = 40
	m.height = 10
	m.hasObs = true
	m.obs = agent.Observation{Mode: "dungeon", Dungeon: &agent.DungeonView{Grid: [][]rune{[]rune("###")}}}
	v := m.View()
	if !strings.Contains(v, "Terminal too small.") || !strings.Contains(v, "Resize to at least 80x24.") {
		t.Fatalf("expected fallback message for small terminal")
	}
}

func TestHeaderConsistencyContainsLabels(t *testing.T) {
	SetPresentationOptions(PresentationOptions{ASCIIMode: false, ColorLevel: ColorTrue})
	in := make(chan tea.Msg, 1)
	m := NewModel(in, &stubNet{}, ModelOptions{})
	m.width = 120
	m.height = 30
	m.energy = 88
	m.hasObs = true
	m.obs = agent.Observation{Mode: "dungeon", Dungeon: &agent.DungeonView{Grid: [][]rune{[]rune("###")}, InstabilityLabel: "CRITICAL", Pressure: 5, MaxPressure: 20, CoreIntegrity: 97, Threat: "HIGH"}}
	v := m.View()
	if !strings.Contains(v, "NIGHTSHADE") || !strings.Contains(v, "DUNGEON") || !strings.Contains(v, "STABILITY:") || !strings.Contains(v, "PHASE:") {
		t.Fatalf("expected header labels to be present, got:\n%s", v)
	}
}

func TestUnicodeFrameBorderRendering(t *testing.T) {
	SetPresentationOptions(PresentationOptions{ASCIIMode: false, ColorLevel: ColorTrue})
	chars := currentFrameCharset()
	if chars.tl != '╭' || chars.bl != '╰' || chars.v != '│' {
		t.Fatalf("expected unicode frame charset")
	}
	in := make(chan tea.Msg, 1)
	m := NewModel(in, &stubNet{}, ModelOptions{})
	m.width = 100
	m.height = 28
	m.energy = 50
	m.hasObs = true
	m.obs = agent.Observation{Mode: "dungeon", Dungeon: &agent.DungeonView{Grid: [][]rune{[]rune("###"), []rune("#@#"), []rune("###")}, InstabilityLabel: "UNSTABLE", Phase: "HUNTER", Pressure: 6, MaxPressure: 20, CoreIntegrity: 62, Threat: "HIGH"}}
	v := m.View()
	if !strings.Contains(v, "NIGHTSHADE") {
		t.Fatalf("expected branded unicode frame output")
	}
}

func TestASCIIBorderFallback(t *testing.T) {
	SetPresentationOptions(PresentationOptions{ASCIIMode: true, ColorLevel: ColorNone})
	defer SetPresentationOptions(PresentationOptions{ASCIIMode: false, ColorLevel: ColorTrue})
	chars := currentFrameCharset()
	if chars.tl != '+' || chars.h != '-' || chars.v != '|' {
		t.Fatalf("expected ASCII frame charset")
	}
	in := make(chan tea.Msg, 1)
	m := NewModel(in, &stubNet{}, ModelOptions{})
	m.width = 100
	m.height = 28
	m.energy = 50
	m.hasObs = true
	m.obs = agent.Observation{Mode: "dungeon", Dungeon: &agent.DungeonView{Grid: [][]rune{[]rune("###")}, InstabilityLabel: "STABLE", Phase: "HUNTER", Pressure: 2, MaxPressure: 20, CoreIntegrity: 90, Threat: "LOW"}}
	v := m.View()
	if !strings.Contains(v, "NIGHTSHADE - DUNGEON") {
		t.Fatalf("expected ASCII frame border characters")
	}
	if strings.Contains(v, "NIGHTSHADE — DUNGEON") {
		t.Fatalf("did not expect unicode frame in ASCII mode")
	}
}

func TestSplashScreenBehavior(t *testing.T) {
	in := make(chan tea.Msg, 1)
	m := NewModel(in, &stubNet{}, ModelOptions{ShowSplash: true})
	m.width = 100
	m.height = 28
	if !strings.Contains(m.View(), "Press any key") {
		t.Fatalf("expected splash prompt")
	}
	nextAny, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	nm := nextAny.(Model)
	if nm.showSplash {
		t.Fatalf("expected splash to dismiss on any key")
	}
}

func TestNoLayoutDriftOnResize(t *testing.T) {
	SetPresentationOptions(PresentationOptions{ASCIIMode: false, ColorLevel: ColorTrue})
	in := make(chan tea.Msg, 1)
	m := NewModel(in, &stubNet{}, ModelOptions{})
	m.hasObs = true
	m.obs = agent.Observation{Mode: "dungeon", Dungeon: &agent.DungeonView{Grid: [][]rune{[]rune("###"), []rune("#@#"), []rune("###")}, InstabilityLabel: "DANGEROUS", Phase: "HUNTER", Pressure: 9, MaxPressure: 20, CoreIntegrity: 54, Threat: "HIGH"}}
	for _, width := range []int{80, 100, 120} {
		m.width = width
		m.height = 24
		out := m.View()
		for _, line := range strings.Split(out, "\n") {
			if displayWidth(line) != width {
				t.Fatalf("line width drift for width=%d got=%d line=%q", width, displayWidth(line), line)
			}
		}
	}
}

func TestMovementChangesRenderedGrid(t *testing.T) {
	SetPresentationOptions(PresentationOptions{ASCIIMode: true, ColorLevel: ColorNone})
	defer SetPresentationOptions(PresentationOptions{ASCIIMode: false, ColorLevel: ColorTrue})
	in := make(chan tea.Msg, 2)
	m := NewModel(in, &stubNet{}, ModelOptions{})
	m.width = 100
	m.height = 28

	obsA := agent.Observation{Mode: "dungeon", Position: core.Position{X: 1, Y: 1}, Dungeon: &agent.DungeonView{Grid: [][]rune{[]rune("#####"), []rune("#...#"), []rune("#####")}, PlayerPosKnown: true, PlayerX: 1, PlayerY: 1, InstabilityLabel: "STABLE", Phase: "HUNTER", Pressure: 2, MaxPressure: 20, CoreIntegrity: 90, Threat: "LOW"}}
	nextA, _ := m.Update(SnapshotMsg{Obs: obsA, Energy: 100})
	mA := nextA.(Model)
	vA := mA.View()

	obsB := agent.Observation{Mode: "dungeon", Position: core.Position{X: 2, Y: 1}, Dungeon: &agent.DungeonView{Grid: [][]rune{[]rune("#####"), []rune("#...#"), []rune("#####")}, PlayerPosKnown: true, PlayerX: 2, PlayerY: 1, InstabilityLabel: "STABLE", Phase: "HUNTER", Pressure: 2, MaxPressure: 20, CoreIntegrity: 90, Threat: "LOW"}}
	nextB, _ := mA.Update(SnapshotMsg{Obs: obsB, Energy: 100})
	mB := nextB.(Model)
	vB := mB.View()

	if vA == vB {
		t.Fatalf("expected rendered output to change with player movement")
	}
	if !strings.Contains(vA, "@") || !strings.Contains(vB, "@") {
		t.Fatalf("expected player glyph to remain visible in both frames")
	}
	if strings.Count(stripANSI(vA), "@") != 1 || strings.Count(stripANSI(vB), "@") != 1 {
		t.Fatalf("expected exactly one player glyph per frame")
	}
}

func TestDungeonOverlayUsesSnapshotPlayerCoordinates(t *testing.T) {
	SetPresentationOptions(PresentationOptions{ASCIIMode: true, ColorLevel: ColorNone})
	defer SetPresentationOptions(PresentationOptions{ASCIIMode: false, ColorLevel: ColorTrue})
	in := make(chan tea.Msg, 1)
	m := NewModel(in, &stubNet{}, ModelOptions{})
	m.width = 100
	m.height = 28
	m.hasObs = true
	m.obs = agent.Observation{
		Mode:     "dungeon",
		Position: core.Position{X: 99, Y: 99},
		Dungeon: &agent.DungeonView{
			Grid:             [][]rune{[]rune("#####"), []rune("#...#"), []rune("#####")},
			PlayerPosKnown:   true,
			PlayerX:          2,
			PlayerY:          1,
			InstabilityLabel: "STABLE",
			Phase:            "HUNTER",
			Pressure:         1,
			MaxPressure:      20,
			CoreIntegrity:    100,
			Threat:           "LOW",
		},
	}
	out := stripANSI(m.View())
	if strings.Count(out, "@") != 1 {
		t.Fatalf("expected exactly one player glyph from snapshot coordinates")
	}
}

func TestDungeonOverlayNoDuplicatePlayerWhenEnemySharesTile(t *testing.T) {
	SetPresentationOptions(PresentationOptions{ASCIIMode: true, ColorLevel: ColorNone})
	defer SetPresentationOptions(PresentationOptions{ASCIIMode: false, ColorLevel: ColorTrue})
	in := make(chan tea.Msg, 1)
	m := NewModel(in, &stubNet{}, ModelOptions{})
	m.width = 100
	m.height = 28
	m.hasObs = true
	m.obs = agent.Observation{
		Mode: "dungeon",
		Dungeon: &agent.DungeonView{
			Grid:             [][]rune{[]rune("#####"), []rune("#...#"), []rune("#####")},
			PlayerPosKnown:   true,
			PlayerX:          2,
			PlayerY:          1,
			Enemies:          []agent.EnemyView{{X: 2, Y: 1, Kind: "HUNTER"}},
			InstabilityLabel: "STABLE",
			Phase:            "HUNTER",
			Pressure:         1,
			MaxPressure:      20,
			CoreIntegrity:    100,
			Threat:           "LOW",
		},
	}
	out := stripANSI(m.View())
	if strings.Count(out, "@") != 1 {
		t.Fatalf("expected exactly one player glyph when enemy overlaps player")
	}
}

func TestUnchangedPlayerPositionKeepsRenderedGrid(t *testing.T) {
	SetPresentationOptions(PresentationOptions{ASCIIMode: true, ColorLevel: ColorNone})
	defer SetPresentationOptions(PresentationOptions{ASCIIMode: false, ColorLevel: ColorTrue})
	in := make(chan tea.Msg, 2)
	m := NewModel(in, &stubNet{}, ModelOptions{})
	m.width = 100
	m.height = 28

	obs := agent.Observation{Mode: "dungeon", Position: core.Position{X: 2, Y: 1}, Dungeon: &agent.DungeonView{Grid: [][]rune{[]rune("#####"), []rune("#...#"), []rune("#####")}, PlayerPosKnown: true, PlayerX: 2, PlayerY: 1, InstabilityLabel: "STABLE", Phase: "HUNTER", Pressure: 2, MaxPressure: 20, CoreIntegrity: 90, Threat: "LOW"}}
	nextA, _ := m.Update(SnapshotMsg{Obs: obs, Energy: 100})
	mA := nextA.(Model)
	vA := mA.View()

	nextB, _ := mA.Update(SnapshotMsg{Obs: obs, Energy: 100})
	mB := nextB.(Model)
	vB := mB.View()

	if vA != vB {
		t.Fatalf("expected identical rendered output when player position is unchanged")
	}
}

func TestPhaseLineNotTruncated(t *testing.T) {
	SetPresentationOptions(PresentationOptions{ASCIIMode: false, ColorLevel: ColorTrue})
	in := make(chan tea.Msg, 1)
	m := NewModel(in, &stubNet{}, ModelOptions{})
	m.width = 80
	m.height = 24
	m.hasObs = true
	m.obs = agent.Observation{Mode: "dungeon", Dungeon: &agent.DungeonView{Grid: [][]rune{[]rune("###")}, InstabilityLabel: "STABLE", Phase: "HUNTER", Pressure: 3, MaxPressure: 20, CoreIntegrity: 88, Threat: "LOW"}}
	v := stripANSI(m.View())
	if !strings.Contains(v, "PHASE: HUNTER") {
		t.Fatalf("expected full PHASE line without clipping")
	}
}
