package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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
