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
		return "move north", true
	case 'a':
		return "move west", true
	case 's':
		return "move south", true
	case 'd':
		return "move east", true
	case '.':
		return "wait", true
	case 'e':
		return "observe", true
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
