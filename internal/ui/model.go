package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/divijg19/Nightshade/internal/agent"
)

type snapshotMsg struct {
	obs    agent.Observation
	energy int
}

type connectionClosedMsg struct{}

type Model struct {
	incoming <-chan tea.Msg
	send     func(string) error

	obs      agent.Observation
	energy   int
	hasObs   bool
	status   string
	showHelp bool

	history      []agent.Observation
	replayCursor int
	inReplay     bool

	lastSignalID string
	pendingDesc  string
	pendingMove  bool
	pendingBaseX int
	pendingBaseY int

	disconnected bool
	quitRequested bool
	width int
	height int
}

func NewModel(incoming <-chan tea.Msg, send func(string) error) Model {
	return Model{incoming: incoming, send: send, history: make([]agent.Observation, 0, 32)}
}

func waitIncoming(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg { return <-ch }
}

func (m Model) Init() tea.Cmd { return waitIncoming(m.incoming) }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		return handleKey(m, msg)
	case snapshotMsg:
		m.obs = msg.obs
		m.energy = msg.energy
		m.hasObs = true
		m.history = append(m.history, msg.obs)
		if len(m.history) > 32 {
			m.history = m.history[len(m.history)-32:]
		}
		m.replayCursor = len(m.history) - 1
		m.inReplay = false
		if m.pendingDesc != "" {
			resolved := "→ " + m.pendingDesc
			if m.pendingMove {
				dx := m.obs.Position.X - m.pendingBaseX
				dy := m.obs.Position.Y - m.pendingBaseY
				if dx < 0 {
					dx = -dx
				}
				if dy < 0 {
					dy = -dy
				}
				if dx+dy != 1 {
					resolved = "Blocked by terrain."
				}
			}
			m.status = resolved
			m.pendingDesc = ""
			m.pendingMove = false
		}
		if m.showHelp {
			m.status = helpStatus()
		}
		return m, waitIncoming(m.incoming)
	case connectionClosedMsg:
		m.disconnected = true
		m.status = "Disconnected from server."
		return m, tea.Quit
	default:
		return m, waitIncoming(m.incoming)
	}
}

func (m Model) View() string {
	if !m.hasObs {
		return "Connecting..."
	}
	return routeView(m)
}

func (m Model) WantsQuit() bool { return m.quitRequested }
func (m Model) WantsReconnect() bool { return m.disconnected && !m.quitRequested }

func helpStatus() string {
	return "Controls: w/a/s/d move  e observe  f ability  . wait\nMore: i introspect  [ ] replay  Ctrl-C quit  ? help"
}

func buildIntrospectionLine(obs agent.Observation) string {
	var total, certain, recent, fading, doubtful int
	hasScars := false
	for _, b := range obs.Known {
		total++
		age := b.Age
		switch {
		case age == 0:
			certain++
		case age >= 1 && age <= agent.CautionThreshold:
			recent++
		case age > agent.CautionThreshold && age <= agent.ParanoiaThreshold:
			fading++
		case age > agent.ParanoiaThreshold:
			doubtful++
		}
		if b.ScarLevel > 0 {
			hasScars = true
		}
	}
	return fmt.Sprintf("Beliefs: %d  Certain:%d Recent:%d Fading:%d Doubtful:%d Scars:%t", total, certain, recent, fading, doubtful, hasScars)
}

func quickDiveSignalID(signals []agent.SignalView) (string, bool) {
	bestIdx := -1
	bestDecay := 1 << 30
	for i, s := range signals {
		if s.Burned || s.Locked {
			continue
		}
		if s.Decay < bestDecay {
			bestDecay = s.Decay
			bestIdx = i
		}
	}
	if bestIdx < 0 {
		return "", false
	}
	return signals[bestIdx].ID, true
}

func canResumeSignal(signals []agent.SignalView, signalID string) bool {
	if signalID == "" {
		return false
	}
	for _, s := range signals {
		if s.ID == signalID && !s.Burned && !s.Locked {
			return true
		}
	}
	return false
}

func asInputKey(msg tea.KeyMsg) string {
	switch msg.Type {
	case tea.KeyEnter:
		return "."
	case tea.KeyCtrlC:
		return "CTRL_C"
	}
	s := strings.TrimSpace(msg.String())
	if len(s) == 1 {
		if s[0] >= 'A' && s[0] <= 'Z' {
			s = strings.ToLower(s)
		}
	}
	return s
}
