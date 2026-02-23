package ui

import (
	"strings"
	"testing"
)

func TestColorDowngradeCompatibility(t *testing.T) {
	SetPresentationOptions(PresentationOptions{ASCIIMode: false, ColorLevel: ColorNone})
	outNone := styleColor(ColorPrimary, "X")
	if strings.Contains(outNone, "\x1b[") {
		t.Fatalf("expected no ANSI in ColorNone mode")
	}

	SetPresentationOptions(PresentationOptions{ASCIIMode: false, ColorLevel: ColorANSI})
	outANSI := styleColor(ColorPrimary, "X")
	if !strings.Contains(outANSI, "\x1b[") {
		t.Fatalf("expected ANSI escape in ColorANSI mode")
	}

	SetPresentationOptions(PresentationOptions{ASCIIMode: false, ColorLevel: Color256})
	out256 := styleColor(ColorPrimary, "X")
	if !strings.Contains(out256, "38;5;") {
		t.Fatalf("expected 256-color escape in Color256 mode")
	}

	SetPresentationOptions(PresentationOptions{ASCIIMode: false, ColorLevel: ColorTrue})
	outTrue := styleColor(ColorPrimary, "X")
	if !strings.Contains(outTrue, "38;2;") {
		t.Fatalf("expected truecolor escape in ColorTrue mode")
	}
}
