package client

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestRecoverAndRestoreTerminalPanicPath(t *testing.T) {
	var out bytes.Buffer
	err := recoverAndRestoreTerminal(&out, errors.New("boom"))
	text := out.String()
	if !strings.Contains(text, "\x1b[?1049l") {
		t.Fatalf("expected alt-screen exit sequence in output")
	}
	if !strings.Contains(text, "\x1b[?25h") {
		t.Fatalf("expected cursor-restore sequence in output")
	}
	if err == nil {
		t.Fatalf("expected panic recovery error")
	}
}
