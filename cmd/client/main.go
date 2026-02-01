package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/divijg19/Nightshade/internal/agent"
	nnet "github.com/divijg19/Nightshade/internal/net"
	"github.com/divijg19/Nightshade/internal/persist"
	"github.com/divijg19/Nightshade/internal/render"

	"golang.org/x/term"
)

type helloMsg struct {
	Type      string `json:"type"`
	PublicKey string `json:"public_key"`
}

type inputMsg struct {
	Type string `json:"type"`
	Key  string `json:"key"`
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
	lastEnergy := 100
	lastParanoia := 3
	lastScars := 0
	var lastObs agent.Observation
	var history []agent.Observation
	replayCursor := 0
	inReplay := false

	stdin := bufio.NewReader(os.Stdin)

	// (render helper removed — rendering is done synchronously per observation)

	// main connect/reconnect loop
	attempts := 0
	start := time.Now()
	for {
		// global timeout: if we haven't established an interface within 10s,
		// give up to avoid long stalls (restore terminal via defer).
		if time.Since(start) > 10*time.Second {
			fmt.Fprintln(os.Stderr, "Timeout: failed to establish connection within 10s. Exiting.")
			return
		}
		conn, err := net.Dial("unix", socket)
		if err != nil {
			attempts++
			// retry after 1s (no UI output here; renderer owns UI)
			if attempts >= 10 {
				// give up after 10 deterministic attempts
				fmt.Fprintln(os.Stderr, "Failed to connect after 10 attempts. Exiting.")
				return
			}
			time.Sleep(1 * time.Second)
			continue
		}
		attempts = 0

		// send hello
		if err := nnet.WriteFrame(conn, helloMsg{Type: "hello", PublicKey: pubB64}); err != nil {
			conn.Close()
			time.Sleep(1 * time.Second)
			continue
		}

		// synchronous processing per connection

		// Synchronous receive loop: receive one observation, render once,
		// then block for a single user input (Enter-terminated) and send it.
		// Enforce a timeout for the first observation to avoid deadlocks
		// when the server and client are both waiting.
		firstFrame := true
	loopConn:
		for {
			var frame map[string]interface{}
			if firstFrame {
				_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
			}
			if err := nnet.ReadFrame(conn, &frame); err != nil {
				// if this was a timeout waiting for the first frame, exit cleanly
				if ne, ok := err.(net.Error); ok && ne.Timeout() && firstFrame {
					term.Restore(int(os.Stdin.Fd()), oldState)
					fmt.Fprintln(os.Stderr, "Timeout waiting for initial observation. Exiting.")
					os.Exit(2)
				}
				// remote closed or other error; break to reconnect
				break loopConn
			}
			if firstFrame {
				// clear deadline after successful first read
				_ = conn.SetReadDeadline(time.Time{})
				firstFrame = false
			}
			if frame["type"] != "obs" {
				continue
			}
			// decode observation
			obsPayload, ok := frame["obs"]
			if !ok {
				continue
			}
			b, err := json.Marshal(obsPayload)
			if err != nil {
				continue
			}
			var obs agent.Observation
			if err := json.Unmarshal(b, &obs); err != nil {
				continue
			}
			// render once
			if e, ok := frame["energy"].(float64); ok {
				lastEnergy = int(e)
			}
			lastObs = obs
			history = append(history, obs)
			if len(history) > 32 {
				history = history[len(history)-32:]
			}
			replayCursor = len(history) - 1
			inReplay = false
			render.RenderTo(os.Stdout, obs, lastEnergy, lastParanoia, lastScars, "", "")

			// now block for one key input; DO NOT print or re-render on keystrokes.
		inputLoop:
			for {
				b, err := stdin.ReadByte()
				if err != nil {
					// treat as disconnect
					break loopConn
				}
				// handle control keys
				if b == 3 || b == 4 || b == 'q' { // CTRL-C, CTRL-D, or 'q'
					// restore terminal, clear screen, and exit
					term.Restore(int(os.Stdin.Fd()), oldState)
					fmt.Fprint(os.Stdout, "\x1b[2J\x1b[H")
					os.Exit(0)
				}

				key, ok := normalizeKey(b)
				if !ok {
					continue
				}

				// client-only commands: introspect and replay navigation
				switch key {
				case 'i':
					eph := buildIntrospectionLine(lastObs)
					render.RenderTo(os.Stdout, lastObs, lastEnergy, lastParanoia, lastScars, "", eph)
					continue
				case '[', ']':
					if len(history) == 0 {
						continue
					}
					inReplay = true
					if key == '[' {
						if replayCursor > 0 {
							replayCursor--
						}
					} else {
						if replayCursor < len(history)-1 {
							replayCursor++
						}
					}
					replayObs := history[replayCursor]
					eph := fmt.Sprintf("Replay tick %d (%d/%d)", replayObs.Tick, replayCursor+1, len(history))
					render.RenderTo(os.Stdout, replayObs, lastEnergy, lastParanoia, lastScars, "", eph)
					continue
				}

				// Exit replay on any action key
				if inReplay {
					inReplay = false
					replayCursor = len(history) - 1
					render.RenderTo(os.Stdout, lastObs, lastEnergy, lastParanoia, lastScars, "", "")
				}

				if actionKey, ok := actionFromKey(key); ok {
					_ = nnet.WriteFrame(conn, inputMsg{Type: "input", Key: actionKey})
					break inputLoop
				}
				// ignore unknown keys and keep waiting for a valid action
			}
		}
		// connection loop ended; close and reconnect
		conn.Close()
		time.Sleep(1 * time.Second)
	}
}
