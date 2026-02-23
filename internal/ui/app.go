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

type AppOptions struct {
	ShowSplash bool
}

var newProgram = tea.NewProgram

func New(network NetworkClient) *App {
	return NewWithOptions(network, AppOptions{})
}

func NewWithOptions(network NetworkClient, opts AppOptions) *App {
	incoming := make(chan tea.Msg, 64)
	m := NewModel(incoming, network, ModelOptions{ShowSplash: opts.ShowSplash})
	p := newProgram(m, tea.WithAltScreen())
	return &App{incoming: incoming, program: p}
}

func (a *App) SendSnapshot(obs agent.Observation, energy int) {
	a.incoming <- SnapshotMsg{Obs: obs, Energy: energy}
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
