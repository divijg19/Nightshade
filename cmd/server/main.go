package main

import (
	"bufio"
	"encoding/base64"
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

	func handleConn(conn net.Conn, agents map[string]*agent.RemoteHuman, mu *sync.Mutex) {
	defer conn.Close()
	// Read hello
	var h helloMsg
	if err := nnet.ReadFrame(conn, &h); err != nil {
		log.Printf("hello read: %v", err)
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
		var st struct{ Energy int `json:"energy"` }
		if err := persist.ReadJSON(statePath, &st); err == nil {
			energy = st.Energy
		}

		// Load memory.json (tiles)
		memoryPath := filepath.Join(agentDir, "memory.json")
		var raw struct {
			Tiles []struct {
				X int `json:"x"`
				Y int `json:"y"`
				Glyph int `json:"glyph"`
				LastSeen int `json:"lastSeen"`
				ScarLevel int `json:"scarLevel"`
			} `json:"tiles"`
		}
		if err := persist.ReadJSON(memoryPath, &raw); err == nil {
			for _, t := range raw.Tiles {
				pos := core.Position{X: t.X, Y: t.Y}
				mt := agent.MemoryTile{
					Tile: core.TileView{Position: pos, Glyph: rune(t.Glyph), Visible: true},
					LastSeen: t.LastSeen,
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

	// Start writer goroutine to push observations to client
	go func() {
		for obs := range rh.SendObservation {
			out := map[string]interface{}{"type": "obs", "visible": obs.Visible, "tick": obs.Tick}
			// best-effort write
			_ = nnet.WriteFrame(conn, out)
		}
	}()

	// Start reader loop for inputs
	dec := bufio.NewReader(conn)
	for {
		var im inputMsg
		if err := nnet.ReadFrame(dec, &im); err != nil {
			return
		}
		if im.Type == "input" {
			// forward key to agent channel (non-blocking)
			select {
			case rh.RecvInput <- im.Key:
			default:
			}
		}
	}
}

func main() {
	socket := defaultSocket()
	os.Remove(socket)
	l, err := net.Listen("unix", socket)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	defer l.Close()
	log.Printf("server listening on %s", socket)

	agents := map[string]*agent.RemoteHuman{}
	var mu sync.Mutex
	started := false
	var rt *runtime.Runtime

	// accept loop
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				log.Printf("accept: %v", err)
				time.Sleep(100 * time.Millisecond)
				continue
			}
			go handleConn(c, agents, &mu)

			// If runtime not started and we have at least one agent, start it.
			mu.Lock()
			if !started && len(agents) > 0 {
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
					for {
						_ = rt.TickOnce()
						time.Sleep(200 * time.Millisecond)
					}
				}()
			}
			mu.Unlock()
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
						"x": mt.Tile.Position.X,
						"y": mt.Tile.Position.Y,
						"glyph": int(mt.Tile.Glyph),
						"lastSeen": mt.LastSeen,
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
