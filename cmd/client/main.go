package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/divijg19/Nightshade/internal/agent"
	nnet "github.com/divijg19/Nightshade/internal/net"
	"github.com/divijg19/Nightshade/internal/persist"
	"github.com/divijg19/Nightshade/internal/render"

	"golang.org/x/term"
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

    // Terminal raw mode to suppress kernel echo and allow controlled prompt
    oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
    if err != nil {
        fmt.Fprintf(os.Stderr, "terminal raw mode failed: %v\n", err)
        return
    }
    defer term.Restore(int(os.Stdin.Fd()), oldState)

    // Ensure ed25519 identity exists and derive AgentID (base64 public key).
    pub, _, pubB64, err := persist.EnsureIdentity()
    if err != nil {
        fmt.Fprintf(os.Stderr, "identity: %v\n", err)
        return
    }
    _ = pub

    // Shared state for rendering/input
    var mu sync.Mutex
    var lastObs agent.Observation
    lastEnergy := 100
    lastParanoia := 3
    lastScars := 0
    inputBuf := ""
    ephemeral := ""

    // helper to render current state (idempotent)
    doRender := func() {
        mu.Lock()
        obs := lastObs
        e := lastEnergy
        p := lastParanoia
        s := lastScars
        prompt := inputBuf
        ep := ephemeral
        mu.Unlock()
        render.RenderTo(os.Stdout, obs, e, p, s, prompt, ep)
    }

    // main connect/reconnect loop
    for {
        conn, err := net.Dial("unix", socket)
        if err != nil {
            // show disconnected message and retry after 1s
            fmt.Fprint(os.Stdout, "\x1b[2J\x1b[H")
            fmt.Fprintln(os.Stdout, "Connection lost. Waiting to reconnect…")
            time.Sleep(1 * time.Second)
            continue
        }

        // send hello
        if err := nnet.WriteFrame(conn, helloMsg{Type: "hello", PublicKey: pubB64}); err != nil {
            conn.Close()
            time.Sleep(1 * time.Second)
            continue
        }

        // channels to signal shutdown of goroutines on disconnect
        done := make(chan struct{})

        // reader goroutine: update lastObs and render
        go func(c net.Conn) {
            for {
                var frame map[string]interface{}
                if err := nnet.ReadFrame(c, &frame); err != nil {
                    close(done)
                    return
                }
                if frame["type"] == "obs" {
                    if obsPayload, ok := frame["obs"]; ok {
                        b, err := json.Marshal(obsPayload)
                        if err == nil {
                            var obs agent.Observation
                            if err := json.Unmarshal(b, &obs); err == nil {
                                mu.Lock()
                                lastObs = obs
                                if e, ok := frame["energy"].(float64); ok {
                                    lastEnergy = int(e)
                                }
                                mu.Unlock()
                                doRender()
                            }
                        }
                    }
                }
            }
        }(conn)

        // input goroutine: read bytes in raw mode and manage prompt
        go func(c net.Conn) {
            buf := make([]byte, 1)
            for {
                n, err := os.Stdin.Read(buf)
                if err != nil || n == 0 {
                    // treat as disconnect
                    close(done)
                    return
                }
                b := buf[0]
                switch b {
                case 13, 10: // enter
                    // send current buffer as input (non-blocking)
                    key := inputBuf
                    if key == "" {
                        mu.Lock()
                        ephemeral = "Invalid input"
                        mu.Unlock()
                        doRender()
                        // clear ephemeral after next render
                        mu.Lock()
                        ephemeral = ""
                        mu.Unlock()
                        inputBuf = ""
                        doRender()
                        continue
                    }
                    _ = nnet.WriteFrame(c, inputMsg{Type: "input", Key: key})
                    inputBuf = ""
                    doRender()
                case 127, 8: // backspace
                    if len(inputBuf) > 0 {
                        inputBuf = inputBuf[:len(inputBuf)-1]
                        doRender()
                    }
                default:
                    // printable ASCII (32-126)
                    if b >= 32 && b <= 126 {
                        inputBuf += string(b)
                        doRender()
                    }
                }
            }
        }(conn)

        // wait for disconnect signal
        <-done
        conn.Close()
        // show disconnected message and retry after 1s deterministically
        fmt.Fprint(os.Stdout, "\x1b[2J\x1b[H")
        fmt.Fprintln(os.Stdout, "Connection lost. Waiting to reconnect…")
        time.Sleep(1 * time.Second)
    }
}
