package main

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestRunDiagnosePrintsExpectedFields(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("COLORTERM", "truecolor")
	t.Setenv("LANG", "en_US.UTF-8")
	t.Setenv("NO_COLOR", "")

	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	runErr := runDiagnose(false, false)
	_ = w.Close()
	os.Stdout = origStdout

	if runErr != nil {
		t.Fatalf("runDiagnose error: %v", runErr)
	}
	out, readErr := io.ReadAll(r)
	if readErr != nil {
		t.Fatalf("read output: %v", readErr)
	}
	text := string(out)
	for _, field := range []string{
		"TERM:",
		"COLORTERM:",
		"Terminal size:",
		"Truecolor detected?:",
		"256-color detected?:",
		"Unicode safe?:",
		"Alt-screen support?:",
		"Color disabled?:",
		"ASCII mode active?:",
	} {
		if !strings.Contains(text, field) {
			t.Fatalf("diagnose output missing %q", field)
		}
	}
}
