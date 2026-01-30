package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"

	nnet "github.com/divijg19/Nightshade/internal/net"
	"github.com/divijg19/Nightshade/internal/persist"
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

    // Reader goroutine: print observations
    go func() {
        for {
            var m map[string]interface{}
            if err := nnet.ReadFrame(conn, &m); err != nil {
                return
            }
            if m["type"] == "obs" {
                fmt.Printf("Tick %v Visible: %v\n", m["tick"], m["visible"])
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
