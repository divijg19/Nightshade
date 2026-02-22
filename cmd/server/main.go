package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"flag"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/divijg19/Nightshade/internal/agent"
	"github.com/divijg19/Nightshade/internal/core"
	nnet "github.com/divijg19/Nightshade/internal/net"
	"github.com/divijg19/Nightshade/internal/persist"
	"github.com/divijg19/Nightshade/internal/runtime"
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

func handleConn(conn net.Conn, agents map[string]*agent.RemoteHuman, cancels map[string]context.CancelFunc, mu *sync.Mutex, rt *runtime.Runtime, startRuntime func()) {
	defer conn.Close()
	// Read hello
	var h helloMsg
	if err := nnet.ReadFrame(conn, &h); err != nil {
		log.Printf("invalid_frame agent=? err=%v", err)
		return
	}
	// Validate that PublicKey is base64 and of correct length for ed25519
	pubB64 := h.PublicKey
	if pubB64 == "" {
		log.Printf("empty public key from client")
		return
	}
	pubBytes, err := base64.StdEncoding.DecodeString(pubB64)
	if err != nil {
		log.Printf("invalid base64 public key: %v", err)
		return
	}
	if len(pubBytes) != 32 {
		log.Printf("invalid public key length: %d", len(pubBytes))
		return
	}

	// AgentID is the base64(public key) string
	agentID := pubB64
	mu.Lock()
	rh, ok := agents[agentID]
	mu.Unlock()
	if !ok {
		// Attempt to rehydrate persisted agent state from disk.
		agentDir := filepath.Join(persist.BaseDir(), "agents", agentID)
		var energy int = agent.MaxEnergy
		mem := agent.NewMemory()

		// Load state.json (energy)
		statePath := filepath.Join(agentDir, "state.json")
		var st struct {
			Energy int `json:"energy"`
		}
		if err := persist.ReadJSON(statePath, &st); err == nil {
			energy = st.Energy
		}

		// Load memory.json (tiles)
		memoryPath := filepath.Join(agentDir, "memory.json")
		var raw struct {
			Tiles []struct {
				X         int `json:"x"`
				Y         int `json:"y"`
				Glyph     int `json:"glyph"`
				LastSeen  int `json:"lastSeen"`
				ScarLevel int `json:"scarLevel"`
			} `json:"tiles"`
		}
		if err := persist.ReadJSON(memoryPath, &raw); err == nil {
			for _, t := range raw.Tiles {
				pos := core.Position{X: t.X, Y: t.Y}
				mt := agent.MemoryTile{
					Tile:      core.TileView{Position: pos, Glyph: rune(t.Glyph), Visible: true},
					LastSeen:  t.LastSeen,
					ScarLevel: t.ScarLevel,
				}
				mem.SetMemoryTile(pos, mt)
			}
		}

		rh = agent.NewRemoteHumanFromExisting(agentID, mem, energy)
		mu.Lock()
		agents[agentID] = rh
		mu.Unlock()
	}
	if startRuntime != nil {
		startRuntime()
	}
	// Manage per-agent connection cancellation so reconnect replaces writers.
	mu.Lock()
	if cancels == nil {
		cancels = map[string]context.CancelFunc{}
	}
	// If an existing connection exists, cancel it (reconnect semantics).
	if cf, exists := cancels[agentID]; exists {
		log.Printf("client_reconnected agent=%s", agentID)
		cf()
	} else {
		log.Printf("client_connected agent=%s", agentID)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancels[agentID] = cancel
	mu.Unlock()

	// Writer goroutine: forwards observations to this connection until ctx canceled.
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case obs, ok := <-rh.SendObservation:
				if !ok {
					return
				}
				out := map[string]interface{}{"type": "obs", "obs": obs, "energy": rh.Energy()}
				if err := nnet.WriteFrame(conn, out); err != nil {
					// write error -> treat as disconnect
					log.Printf("client_disconnected agent=%s reason=error", agentID)
					cancel()
					return
				}
			}
		}
	}()

	// mark this RemoteHuman as connected for runtime blocking reads
	rh.SetConnected(true)

	// Bootstrap observation: send a single pre-tick snapshot to the client
	// so the client can render once before waiting for input. This must not
	// mutate cognition or advance ticks. Prefer a runtime snapshot when
	// available; otherwise synthesize a minimal zero snapshot.
	var snap runtime.Snapshot
	var hasSnap bool
	if rt != nil {
		if s, ok := rt.SnapshotForDebug(agentID); ok {
			snap = s
			hasSnap = true
		}
	}
	if !hasSnap {
		snap = runtime.Snapshot{Tick: 0, Position: core.Position{X: 0, Y: 0}, Visible: nil}
	}
	obs := agent.Observation{
		Visible:  snap.Visible,
		Known:    []agent.Belief{},
		Tick:     snap.Tick,
		Position: snap.Position,
		Presence: nil,
	}
	// Copy presentation-only metadata without mutating agent cognition.
	obs.Mode = snap.Mode
	if len(snap.Board.Signals) > 0 {
		b := snap.Board
		obs.Board = &b
	}
	if snap.Dungeon.ExitStability > 0 || snap.Dungeon.Pressure > 0 || snap.Dungeon.AnchorType != "" {
		d := snap.Dungeon
		obs.Dungeon = &d
		if obs.Mode == "" {
			obs.Mode = "dungeon"
		}
	}
	select {
	case rh.SendObservation <- obs:
	default:
	}

	// Reader loop for inputs
	dec := bufio.NewReader(conn)
	for {
		var im inputMsg
		if err := nnet.ReadFrame(dec, &im); err != nil {
			// Connection read error -> disconnect
			reason := "eof"
			if !os.IsTimeout(err) {
				reason = "invalid"
			}
			log.Printf("client_disconnected agent=%s reason=%s", agentID, reason)
			// cancel writer
			cancel()
			return
		}
		switch im.Type {
		case "input":
			// forward key to agent channel (non-blocking)
			select {
			case rh.RecvInput <- im.Key:
			default:
			}
		case "disconnect":
			log.Printf("client_disconnected agent=%s reason=client_quit", agentID)
			cancel()
			return
		default:
			// unknown frame type -> log and ignore
			log.Printf("invalid_frame agent=%s err=unknown_type", agentID)
		}
	}
}

func main() {
	dev := flag.Bool("dev", false, "enable dev mode: faster ticks, verbose logs")
	flag.Parse()

	socket := defaultSocket()
	os.Remove(socket)
	l, err := net.Listen("unix", socket)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	defer l.Close()
	log.Printf("server listening on %s", socket)

	agents := map[string]*agent.RemoteHuman{}
	cancels := map[string]context.CancelFunc{}
	var mu sync.Mutex
	started := false
	var rt *runtime.Runtime

	startRuntimeIfNeeded := func() {
		mu.Lock()
		defer mu.Unlock()
		if started || len(agents) == 0 {
			return
		}
		started = true
		// Build agent slice
		list := make([]agent.Agent, 0, len(agents)+1)
		for _, a := range agents {
			list = append(list, a)
		}
		// Add one oscillating NPC so world moves
		list = append(list, agent.NewOscillating("npc-osc"))
		rt = runtime.New(list)
		// Start tick loop honoring existing runtime.TickOnce
		go func() {
			var delay time.Duration = 200 * time.Millisecond
			if *dev {
				delay = 50 * time.Millisecond
			}
			for {
				if *dev {
					log.Printf("tick_start")
				}
				_ = rt.TickOnce()
				if *dev {
					log.Printf("tick_end")
				}
				time.Sleep(delay)
			}
		}()
	}

	// accept loop
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				log.Printf("accept: %v", err)
				time.Sleep(100 * time.Millisecond)
				continue
			}
			go handleConn(c, agents, cancels, &mu, rt, startRuntimeIfNeeded)
		}
	}()

	// simple persistence loop: flush agents to disk periodically
	for {
		mu.Lock()
		for id, a := range agents {
			agentDir := filepath.Join(persist.BaseDir(), "agents", id)
			// persist state.json
			st := map[string]interface{}{"energy": a.Energy()}
			_ = persist.WriteJSON(filepath.Join(agentDir, "state.json"), st)

			// persist memory.json in deterministic shape
			tiles := []map[string]int{}
			if a.Memory() != nil {
				for _, mt := range a.Memory().All() {
					tiles = append(tiles, map[string]int{
						"x":         mt.Tile.Position.X,
						"y":         mt.Tile.Position.Y,
						"glyph":     int(mt.Tile.Glyph),
						"lastSeen":  mt.LastSeen,
						"scarLevel": mt.ScarLevel,
					})
				}
			}
			memOut := map[string]interface{}{"tiles": tiles}
			_ = persist.WriteJSON(filepath.Join(agentDir, "memory.json"), memOut)
		}
		mu.Unlock()
		time.Sleep(1 * time.Second)
	}
}
