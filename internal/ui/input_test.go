package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/divijg19/Nightshade/internal/agent"
)

func TestCtrlCQuitsImmediatelyWithSplash(t *testing.T) {
	in := make(chan tea.Msg, 1)
	m := NewModel(in, &stubNet{}, ModelOptions{ShowSplash: true})
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	next := updated.(Model)
	if !next.quitRequested {
		t.Fatalf("expected quitRequested true on first Ctrl+C")
	}
	if cmd == nil {
		t.Fatalf("expected quit command on first Ctrl+C")
	}
}

func TestBoardModeMovementKeyDoesNotDispatch(t *testing.T) {
	in := make(chan tea.Msg, 1)
	net := &stubNet{}
	m := NewModel(in, net, ModelOptions{})
	m.hasObs = true
	m.obs = agent.Observation{Mode: "board", Board: &agent.BoardView{Signals: []agent.SignalView{{ID: "S000", Type: "NULL"}}}}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	next := updated.(Model)
	if len(net.sent) != 0 {
		t.Fatalf("expected no runtime input dispatch in board mode, got %v", net.sent)
	}
	if next.pendingMove {
		t.Fatalf("expected no pending move in board mode")
	}
}
