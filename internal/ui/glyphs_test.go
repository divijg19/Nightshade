package ui

import (
	"strings"
	"testing"
)

func TestFrozenGlyphFallbackMap(t *testing.T) {
	SetPresentationOptions(PresentationOptions{ASCIIMode: true, ColorLevel: ColorNone})
	defer SetPresentationOptions(PresentationOptions{ASCIIMode: false, ColorLevel: ColorTrue})

	cases := map[rune]rune{
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
	for unicodeRune, asciiRune := range cases {
		got := normalizeGlyph(unicodeRune)
		if got != asciiRune {
			t.Fatalf("expected fallback %q for %q, got %q", string(asciiRune), string(unicodeRune), string(got))
		}
	}
}

func TestWidthSafeSanitizationPreventsDrift(t *testing.T) {
	SetPresentationOptions(PresentationOptions{ASCIIMode: false, ColorLevel: ColorNone})
	defer SetPresentationOptions(PresentationOptions{ASCIIMode: false, ColorLevel: ColorTrue})

	line := "A◉B⚔C"
	sanitized := sanitizeGlyphsPreserveANSI(line)
	if displayWidth(sanitized) != len([]rune(sanitized)) {
		t.Fatalf("expected fixed-width runes after sanitization, got %q", sanitized)
	}
	fit := fitLine(line, 12)
	if displayWidth(fit) != 12 {
		t.Fatalf("expected fitLine to produce exact width; got %d", displayWidth(fit))
	}
}

func TestNoColorModeDisablesANSISequences(t *testing.T) {
	SetPresentationOptions(PresentationOptions{ASCIIMode: false, ColorLevel: ColorNone})
	defer SetPresentationOptions(PresentationOptions{ASCIIMode: false, ColorLevel: ColorTrue})

	out := strongThreat("CRITICAL", "-")
	if strings.Contains(out, "\x1b[") {
		t.Fatalf("expected no ANSI escapes in no-color mode, got %q", out)
	}
}
