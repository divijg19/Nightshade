package ui

import (
	"strings"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
)

var unicodeToASCIIGlyph = map[rune]rune{
	'◉': 'O',
	'◇': 'X',
	'▲': '^',
	'⚔': 'H',
	'☣': 'S',
	'✶': 'W',
	'✕': 'x',
	'✚': '+',
	'◌': '.',
	'☉': '*',
}

func normalizeGlyph(r rune) rune {
	opts := currentPresentationOptions()
	if a, ok := unicodeToASCIIGlyph[r]; ok {
		if opts.ASCIIMode {
			return a
		}
		if runewidth.RuneWidth(r) != 1 {
			return a
		}
		return r
	}

	if opts.ASCIIMode {
		if r < 32 || r > 126 {
			return '?'
		}
		if runewidth.RuneWidth(r) != 1 {
			return '?'
		}
		return r
	}

	if runewidth.RuneWidth(r) != 1 {
		return '?'
	}
	return r
}

func sanitizeGlyphsPreserveANSI(s string) string {
	if s == "" {
		return s
	}

	var b strings.Builder
	b.Grow(len(s))

	for i := 0; i < len(s); {
		if s[i] == '\x1b' {
			j := i + 1
			for j < len(s) {
				if s[j] == 'm' {
					j++
					break
				}
				j++
			}
			b.WriteString(s[i:j])
			i = j
			continue
		}

		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			b.WriteRune('?')
			i++
			continue
		}
		b.WriteRune(normalizeGlyph(r))
		i += size
	}

	return b.String()
}
