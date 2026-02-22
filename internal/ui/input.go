package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

func handleKey(m Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := asInputKey(msg)
	if key == "CTRL_C" {
		if m.network != nil {
			_ = m.network.Disconnect()
		}
		m.quitRequested = true
		return m, tea.Quit
	}
	if key == "?" {
		m.showHelp = !m.showHelp
		if m.showHelp {
			m.status = helpStatus()
		} else {
			m.status = ""
		}
		return m, nil
	}
	if key == "i" {
		m.status = buildIntrospectionLine(m.obs)
		return m, nil
	}
	if key == "[" || key == "]" {
		if len(m.history) == 0 {
			return m, nil
		}
		m.inReplay = true
		if key == "[" {
			if m.replayCursor > 0 {
				m.replayCursor--
			}
		} else {
			if m.replayCursor < len(m.history)-1 {
				m.replayCursor++
			}
		}
		r := m.history[m.replayCursor]
		m.status = fmt.Sprintf("Replay tick %d (%d/%d)", r.Tick, m.replayCursor+1, len(m.history))
		m.obs = r
		return m, nil
	}
	if m.inReplay {
		m.inReplay = false
		m.replayCursor = len(m.history) - 1
	}

	if m.obs.Mode == "board" && m.obs.Board != nil {
		switch key {
		case "q":
			sid, ok := quickDiveSignalID(m.obs.Board.Signals)
			if !ok {
				m.status = "No valid signal for Quick Dive."
				return m, nil
			}
			m.lastSignalID = sid
			m.pendingDesc = "Quick Dive"
			if m.network != nil {
				_ = m.network.SendInput("ENTER_SIGNAL " + sid)
			}
			return m, nil
		case "r":
			if !canResumeSignal(m.obs.Board.Signals, m.lastSignalID) {
				if m.lastSignalID == "" {
					m.status = "No previous signal to resume."
				} else {
					m.status = "Resume unavailable."
				}
				return m, nil
			}
			m.pendingDesc = "Resume Last"
			if m.network != nil {
				_ = m.network.SendInput("ENTER_SIGNAL " + m.lastSignalID)
			}
			return m, nil
		}
		if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
			idx := int(key[0] - '1')
			if idx >= 0 && idx < len(m.obs.Board.Signals) {
				sid := m.obs.Board.Signals[idx].ID
				m.lastSignalID = sid
				m.pendingDesc = "enter signal " + sid
				if m.network != nil {
					_ = m.network.SendInput("ENTER_SIGNAL " + sid)
				}
				return m, nil
			}
		}
	}

	switch key {
	case "w", "a", "s", "d", "e", ".", "f", "1", "2", "3":
		if key == "w" || key == "a" || key == "s" || key == "d" {
			m.pendingMove = true
			m.pendingBaseX = m.obs.Position.X
			m.pendingBaseY = m.obs.Position.Y
		}
		desc := map[string]string{"w": "Move NORTH (-1)", "a": "Move WEST (-1)", "s": "Move SOUTH (-1)", "d": "Move EAST (-1)", "e": "Observe (-1)", ".": "Wait (+1)", "f": "Ability (F)", "1": "Path: Stabilizer", "2": "Path: Harvester", "3": "Path: Aggressor"}[key]
		m.pendingDesc = desc
		if m.network != nil {
			_ = m.network.SendInput(key)
		}
		return m, nil
	default:
		m.status = fmt.Sprintf("Unknown key: %q  (? for help)", key)
		return m, nil
	}
}
