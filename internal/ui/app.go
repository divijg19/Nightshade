package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/divijg19/Nightshade/internal/agent"
)

type ExitReason int

const (
	ExitReconnect ExitReason = iota
	ExitQuit
)

type App struct {
	incoming chan tea.Msg
	program  *tea.Program
}

func New(send func(string) error) *App {
	incoming := make(chan tea.Msg, 64)
	m := NewModel(incoming, send)
	p := tea.NewProgram(m, tea.WithAltScreen())
	return &App{incoming: incoming, program: p}
}

func (a *App) SendSnapshot(obs agent.Observation, energy int) {
	a.incoming <- snapshotMsg{obs: obs, energy: energy}
}

func (a *App) NotifyConnectionClosed() {
	a.incoming <- connectionClosedMsg{}
}

func (a *App) Run() (ExitReason, error) {
	m, err := a.program.Run()
	if err != nil {
		return ExitReconnect, err
	}
	finalModel, ok := m.(Model)
	if !ok {
		return ExitReconnect, nil
	}
	if finalModel.WantsQuit() {
		return ExitQuit, nil
	}
	return ExitReconnect, nil
}
