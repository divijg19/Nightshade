package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"log"
	"net"
	"os"
	"path/filepath"

	"github.com/divijg19/Nightshade/internal/agent"
	nnet "github.com/divijg19/Nightshade/internal/net"
	"github.com/divijg19/Nightshade/internal/persist"
	"github.com/divijg19/Nightshade/internal/render"
)

type helloMsg struct {
    Type string `json:"type"`
    PublicKey string `json:"public_key"`
}

type inputMsg struct {
    Type string `json:"type"`
    Key string `json:"key"`
}

func defaultSocket() string {
    if s := os.Getenv("NIGHTSHADE_SOCKET"); s != "" {
        return s
    }
    return filepath.Join(persist.BaseDir(), "socket")
}

func main() {
    socket := defaultSocket()
    flag.Parse()

    conn, err := net.Dial("unix", socket)
    if err != nil {
        log.Fatalf("dial: %v", err)
    }
    defer conn.Close()

    // Ensure ed25519 identity exists and derive AgentID (base64 public key).
    pub, _, pubB64, err := persist.EnsureIdentity()
    if err != nil {
        log.Fatalf("identity: %v", err)
    }
    _ = pub // pub bytes unused locally

    // Send hello with base64(public key) as PublicKey
    if err := nnet.WriteFrame(conn, helloMsg{Type: "hello", PublicKey: pubB64}); err != nil {
        log.Fatalf("hello write: %v", err)
    }

    // Reader goroutine: full-screen redraw per observation
    go func() {
        for {
            var frame map[string]interface{}
            if err := nnet.ReadFrame(conn, &frame); err != nil {
                return
            }
            if frame["type"] == "obs" {
                // Re-marshal the obs payload and unmarshal into agent.Observation
                if obsPayload, ok := frame["obs"]; ok {
                    b, err := json.Marshal(obsPayload)
                    if err != nil {
                        continue
                    }
                    var obs agent.Observation
                    if err := json.Unmarshal(b, &obs); err != nil {
                        continue
                    }
                    // Render (use energy from frame if present)
                    energy := 100
                    if e, ok := frame["energy"].(float64); ok {
                        energy = int(e)
                    }
                    paranoia := 3
                    scars := 0
                    render.RenderTo(os.Stdout, obs, energy, paranoia, scars, "")
                }
            }
        }
    }()

    // Input loop: read a line and send as input frames
    stdin := bufio.NewScanner(os.Stdin)
    for stdin.Scan() {
        key := stdin.Text()
        _ = nnet.WriteFrame(conn, inputMsg{Type: "input", Key: key})
    }
}
