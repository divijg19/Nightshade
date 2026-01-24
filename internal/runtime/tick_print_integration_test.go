package runtime

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/divijg19/Nightshade/internal/agent"
)

func TestTickOnce_PrintsPerAgentSnapshot(t *testing.T) {
    a1 := agent.NewScripted("A")
    a2 := agent.NewOscillating("B")

    rt := New([]agent.Agent{a1, a2})

    // Capture stdout
    old := os.Stdout
    r, w, _ := os.Pipe()
    os.Stdout = w

    // Run one tick which should trigger debug prints for both agents
    rt.TickOnce()

    w.Close()
    var buf bytes.Buffer
    _, _ = buf.ReadFrom(r)
    os.Stdout = old

    out := buf.String()

    wantA := "Tick 0 Agent A Position=(0,0) Health=0 Energy=0\n"
    if !strings.Contains(out, wantA) {
        t.Fatalf("expected debug output for agent A; got:\n%v", out)
    }

    wantB := "Tick 0 Agent B Position=(1,0) Health=0 Energy=0\n"
    if !strings.Contains(out, wantB) {
        t.Fatalf("expected debug output for agent B; got:\n%v", out)
    }
}
