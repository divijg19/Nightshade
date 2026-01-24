package runtime

import (
	"bytes"
	"os"
	"testing"
)

func TestPrintDebugSnapshot_Output(t *testing.T) {
    // Prepare a snapshot with known values
    snap := Snapshot{
        Tick:   7,
        SelfID: "AgentX",
        Position: Position{
            X: 3,
            Y: 4,
        },
        Health: 10,
        Energy: 20,
    }

    // Capture stdout
    old := os.Stdout
    r, w, _ := os.Pipe()
    os.Stdout = w

    printDebugSnapshot(snap)

    w.Close()
    var buf bytes.Buffer
    _, _ = buf.ReadFrom(r)
    os.Stdout = old

    out := buf.String()
    want := "Tick 7 Agent AgentX Position=(3,4) Health=10 Energy=20\n"
    if out != want {
        t.Fatalf("unexpected output: got %q, want %q", out, want)
    }
}
