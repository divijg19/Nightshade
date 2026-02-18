package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/core"
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
	socketFlag := flag.String("socket", defaultSocket(), "unix socket path (or set NIGHTSHADE_SOCKET)")
	minimalFlag := flag.Bool("minimal", false, "use minimal UI framing")
	flag.Parse()
	socket := *socketFlag

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
	lastEnergy := 0
	haveEnergy := false
	lastParanoia := 3
	lastScars := 0
	var lastObs agent.Observation
	var history []agent.Observation
	replayCursor := 0
	inReplay := false
	showHelp := false

	opts := render.Options{Minimal: *minimalFlag}

	type pendingAction struct {
		key     rune
		desc    string
		basePos core.Position
	}
	var pending *pendingAction
	energyDeltaToShow := 0
	lastSignalID := ""

	stdin := bufio.NewReader(os.Stdin)

	helpStatus := func() string {
		return "Controls: w/a/s/d move  e observe  . wait\nMore: i introspect  [ ] replay  Ctrl-C quit  ? help"
	}

	promptHint := func() string {
		if inReplay {
			return "REPLAY  (? help)"
		}
		if showHelp {
			return "(? hide help)"
		}
		return "(? help)"
	}

	renderNow := func(obs agent.Observation, status string, pulse bool) {
		if pulse && strings.TrimSpace(status) != "" {
			parts := strings.SplitN(status, "\n", 2)
			parts[0] = "\x1b[7m" + parts[0] + "\x1b[0m"
			status = strings.Join(parts, "\n")
		}
		render.RenderToWithOptions(os.Stdout, obs, lastEnergy, lastParanoia, lastScars, promptHint(), status, opts)
	}

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
			prevEnergy := lastEnergy
			if e, ok := frame["energy"].(float64); ok {
				lastEnergy = int(e)
				if haveEnergy {
					if prevEnergy != lastEnergy {
						energyDeltaToShow = lastEnergy - prevEnergy
					}
				} else {
					haveEnergy = true
					energyDeltaToShow = 0
				}
			}
			lastObs = obs
			history = append(history, obs)
			if len(history) > 32 {
				history = history[len(history)-32:]
			}
			replayCursor = len(history) - 1
			inReplay = false

			status := ""
			if showHelp {
				status = helpStatus()
			} else if pending != nil {
				// Resolve action feedback only when the new observation confirms it.
				resolved := "→ " + pending.desc
				if pending.key == 'w' || pending.key == 'a' || pending.key == 's' || pending.key == 'd' {
					dx := obs.Position.X - pending.basePos.X
					dy := obs.Position.Y - pending.basePos.Y
					if dx < 0 {
						dx = -dx
					}
					if dy < 0 {
						dy = -dy
					}
					if dx+dy != 1 {
						resolved = "Blocked by terrain."
					}
				} else {
					resolved = "→ " + pending.desc
				}
				status = resolved
				if energyDeltaToShow != 0 {
					status = fmt.Sprintf("%s  Energy %d/100 (%+d)", status, lastEnergy, energyDeltaToShow)
				}
				energyDeltaToShow = 0
				pending = nil
			} else if energyDeltaToShow != 0 {
				// Keep the delta for the next acknowledged action, but don't spam.
				energyDeltaToShow = 0
			}

			renderNow(obs, status, false)

			// now block for one key input; DO NOT print or re-render on keystrokes.
		inputLoop:
			for {
				b, err := stdin.ReadByte()
				if err != nil {
					// treat as disconnect
					break loopConn
				}
				// handle control keys
				if b == 3 || b == 4 { // CTRL-C or CTRL-D
					// restore terminal, clear screen, and exit
					term.Restore(int(os.Stdin.Fd()), oldState)
					fmt.Fprint(os.Stdout, "\x1b[2J\x1b[H")
					os.Exit(0)
				}

				key, ok := normalizeKey(b)
				if !ok {
					continue
				}

				if key == '?' {
					showHelp = !showHelp
					status := ""
					if showHelp {
						status = helpStatus()
					}
					renderNow(lastObs, status, false)
					continue
				}

				// client-only commands: introspect and replay navigation
				switch key {
				case 'i':
					renderNow(lastObs, buildIntrospectionLine(lastObs), false)
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
					status := fmt.Sprintf("Replay tick %d (%d/%d)", replayObs.Tick, replayCursor+1, len(history))
					renderNow(replayObs, status, false)
					continue
				}

				// Exit replay on any action key
				if inReplay {
					inReplay = false
					replayCursor = len(history) - 1
					renderNow(lastObs, "", false)
				}

				// v0.3.0: signal selection -> enter signal by number.
				if lastObs.Mode == "board" && lastObs.Board != nil {
					if key == 'q' {
						sid, ok := quickDiveSignalID(lastObs.Board.Signals)
						if !ok {
							renderNow(lastObs, "No valid signal for Quick Dive.", true)
							continue
						}
						lastSignalID = sid
						pending = &pendingAction{key: 'e', desc: "Quick Dive", basePos: lastObs.Position}
						renderNow(lastObs, "→ Quick Dive", true)
						_ = nnet.WriteFrame(conn, inputMsg{Type: "input", Key: "ENTER_SIGNAL " + sid})
						break inputLoop
					}

					if key == 'r' {
						if !canResumeSignal(lastObs.Board.Signals, lastSignalID) {
							if lastSignalID == "" {
								renderNow(lastObs, "No previous signal to resume.", true)
								continue
							}
							renderNow(lastObs, "Resume unavailable.", true)
							continue
						}
						pending = &pendingAction{key: 'e', desc: "Resume Last", basePos: lastObs.Position}
						renderNow(lastObs, "→ Resume Last", true)
						_ = nnet.WriteFrame(conn, inputMsg{Type: "input", Key: "ENTER_SIGNAL " + lastSignalID})
						break inputLoop
					}

					if key >= '1' && key <= '9' {
						idx := int(key - '1')
						if idx >= 0 && idx < len(lastObs.Board.Signals) {
							if showHelp {
								showHelp = false
							}
							sid := lastObs.Board.Signals[idx].ID
							lastSignalID = sid
							cmd := "ENTER_SIGNAL " + sid
							desc := "enter signal " + sid
							pending = &pendingAction{key: 'e', desc: desc, basePos: lastObs.Position}
							renderNow(lastObs, "→ "+desc, true)
							_ = nnet.WriteFrame(conn, inputMsg{Type: "input", Key: cmd})
							break inputLoop
						}
					}
				}

				if actionKey, ok := actionFromKey(key); ok {
					if showHelp {
						showHelp = false
					}
					desc, _ := describeActionKey(key)
					pending = &pendingAction{key: key, desc: desc, basePos: lastObs.Position}
					renderNow(lastObs, "→ "+desc, true)
					_ = nnet.WriteFrame(conn, inputMsg{Type: "input", Key: actionKey})
					break inputLoop
				}
				// immediate rejection feedback for invalid keys (no tick)
				renderNow(lastObs, fmt.Sprintf("Unknown key: %q  (? for help)", key), true)
			}
		}
		// connection loop ended; close and reconnect
		conn.Close()
		time.Sleep(1 * time.Second)
	}
}
