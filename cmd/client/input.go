package main

import (
	"fmt"

	"github.com/divijg19/Nightshade/internal/agent"
)

func normalizeKey(b byte) (rune, bool) {
	// Enter -> WAIT
	if b == 13 || b == 10 {
		return '.', true
	}
	if b < 32 || b > 126 {
		return 0, false
	}
	if b >= 'A' && b <= 'Z' {
		b = b + ('a' - 'A')
	}
	return rune(b), true
}

func actionFromKey(r rune) (string, bool) {
	switch r {
	case 'w', 'a', 's', 'd', 'e', '.':
		return string(r), true
	default:
		return "", false
	}
}

func describeActionKey(r rune) (string, bool) {
	switch r {
	case 'w':
		return "Move NORTH (-1)", true
	case 'a':
		return "Move WEST (-1)", true
	case 's':
		return "Move SOUTH (-1)", true
	case 'd':
		return "Move EAST (-1)", true
	case '.':
		return "Wait (+1)", true
	case 'e':
		return "Observe (-1)", true
	default:
		return "", false
	}
}

func movementDelta(r rune) (dx int, dy int, ok bool) {
	switch r {
	case 'w':
		return 0, -1, true
	case 'a':
		return -1, 0, true
	case 's':
		return 0, 1, true
	case 'd':
		return 1, 0, true
	default:
		return 0, 0, false
	}
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
