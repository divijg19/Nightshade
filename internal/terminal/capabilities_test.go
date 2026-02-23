package terminal

import (
	"os"
	"strings"
	"testing"
)

func TestDetectRespectsNoColor(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("COLORTERM", "truecolor")
	t.Setenv("NO_COLOR", "1")
	t.Setenv("LANG", "en_US.UTF-8")

	caps := Detect(Options{})
	if !caps.ColorDisabled {
		t.Fatalf("expected color disabled when NO_COLOR is set")
	}
	if caps.ColorLevel != ColorNone {
		t.Fatalf("expected ColorNone, got %v", caps.ColorLevel)
	}
}

func TestDiagnosticLinesContainRequiredFields(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("COLORTERM", "truecolor")
	t.Setenv("LANG", "en_US.UTF-8")
	_ = os.Unsetenv("NO_COLOR")

	lines := DiagnosticLines(Detect(Options{}))
	joined := strings.Join(lines, "\n")
	required := []string{
		"TERM:",
		"COLORTERM:",
		"Terminal size:",
		"Truecolor detected?:",
		"256-color detected?:",
		"Unicode safe?:",
		"Alt-screen support?:",
		"Color disabled?:",
		"ASCII mode active?:",
	}
	for _, item := range required {
		if !strings.Contains(joined, item) {
			t.Fatalf("expected diagnostics to contain %q", item)
		}
	}
}
