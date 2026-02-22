package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewConfiguresProgramOption(t *testing.T) {
	orig := newProgram
	defer func() { newProgram = orig }()

	capturedOpts := 0
	newProgram = func(model tea.Model, opts ...tea.ProgramOption) *tea.Program {
		capturedOpts = len(opts)
		return nil
	}

	app := New(&stubNet{})
	if app == nil {
		t.Fatalf("expected app")
	}
	if capturedOpts == 0 {
		t.Fatalf("expected program options to be set")
	}
}
