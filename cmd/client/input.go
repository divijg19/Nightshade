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
